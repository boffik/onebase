package backup

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
)

// TestCreateAutoBackup_S3EndToEnd drives the real path: dumpAutoTarget dumps a
// live SQLite DB, the real objstore client signs the request (SigV4) and PUTs it
// over HTTP to a stand-in S3 server. No fakes on the code-under-test side.
func TestCreateAutoBackup_S3EndToEnd(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "live.db")

	db, err := storage.ConnectSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	if _, err := db.Exec(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	db.Close()

	type captured struct {
		method, path, auth, sha string
		body                    []byte
	}
	got := make(chan captured, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		got <- captured{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			sha:    r.Header.Get("X-Amz-Content-Sha256"),
			body:   buf.Bytes(),
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &project.BackupConfig{
		Directory: filepath.Join(dir, "backups"),
		S3: &project.S3Config{
			Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
			Bucket:    "backups",
			Prefix:    "int/",
			AccessKey: "AKIA_TEST",
			SecretKey: "secret",
			UseSSL:    ptrFalse(),
			PathStyle: ptrTrue(),
		},
	}
	target := AutoTarget{DBType: "sqlite", SQLitePath: dbPath, ProjectDir: dir}

	// Public entry point → uses the real newObjectStore factory.
	localPath, err := CreateAutoBackup(ctx, cfg, target)
	if err != nil {
		t.Fatalf("CreateAutoBackup: %v", err)
	}

	select {
	case c := <-got:
		if c.method != http.MethodPut {
			t.Errorf("method = %s, want PUT", c.method)
		}
		wantPath := "/backups/int/" + filepath.Base(localPath)
		if c.path != wantPath {
			t.Errorf("path = %s, want %s", c.path, wantPath)
		}
		if !strings.HasPrefix(c.auth, "AWS4-HMAC-SHA256 Credential=AKIA_TEST/") {
			t.Errorf("authorization = %q", c.auth)
		}
		if c.sha != "UNSIGNED-PAYLOAD" {
			t.Errorf("content-sha = %q", c.sha)
		}
		// The uploaded bytes must be a real SQLite backup.
		if !bytes.HasPrefix(c.body, []byte("SQLite format 3\x00")) {
			t.Errorf("uploaded body is not a SQLite file (len=%d)", len(c.body))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("S3 server never received the upload")
	}
}

func ptrTrue() *bool  { b := true; return &b }
func ptrFalse() *bool { b := false; return &b }
