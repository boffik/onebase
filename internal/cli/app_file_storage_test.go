package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
)

func boolPtr(b bool) *bool { return &b }

// fakeS3Server is a tiny in-memory S3 endpoint: PUT stores, GET returns, DELETE
// removes, keyed by request path.
func fakeS3Server(objects map[string][]byte, mu *sync.Mutex) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			objects[r.URL.Path] = body
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			b, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
			w.WriteHeader(http.StatusOK)
			w.Write(b)
		case http.MethodDelete:
			delete(objects, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

// TestApplyFileStorageS3_EndToEnd drives the real objstore client (SigV4 + HTTP)
// through the storage blob API: applyFileStorageS3 injects it, then a
// PutBlob→OpenBlob→DeleteBlob cycle round-trips through the stand-in S3 server.
func TestApplyFileStorageS3_EndToEnd(t *testing.T) {
	ctx := context.Background()
	objects := map[string][]byte{}
	var mu sync.Mutex
	srv := fakeS3Server(objects, &mu)
	defer srv.Close()

	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "e2e.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	if err := db.EnsureBlobTable(ctx); err != nil {
		t.Fatalf("EnsureBlobTable: %v", err)
	}

	appCfg := &project.AppConfig{FileStorage: &project.FileStorageConfig{S3: &project.S3Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "bucket",
		Prefix:    "px/",
		AccessKey: "AKIA_TEST",
		SecretKey: "secret",
		UseSSL:    boolPtr(false),
		PathStyle: boolPtr(true),
	}}}
	if err := applyFileStorageS3(db, appCfg); err != nil {
		t.Fatalf("applyFileStorageS3: %v", err)
	}
	if err := db.SaveFileStorageMode(ctx, storage.FileStorageS3); err != nil {
		t.Fatalf("SaveFileStorageMode: %v", err)
	}

	payload := []byte("\x89PNG real end-to-end image bytes")
	b, err := db.PutBlob(ctx, "image/png", bytes.NewReader(payload), 1<<20, storage.BlobOwner{})
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	wantPath := "/bucket/px/blobs/" + b.ID.String()
	mu.Lock()
	stored, ok := objects[wantPath]
	mu.Unlock()
	if !ok || !bytes.Equal(stored, payload) {
		t.Fatalf("object not stored at %s (ok=%v len=%d)", wantPath, ok, len(stored))
	}

	_, rc, err := db.OpenBlob(ctx, b.ID)
	if err != nil {
		t.Fatalf("OpenBlob: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, payload) {
		t.Fatalf("OpenBlob content mismatch: %q", got)
	}

	if err := db.DeleteBlob(ctx, b.ID); err != nil {
		t.Fatalf("DeleteBlob: %v", err)
	}
	mu.Lock()
	_, stillThere := objects[wantPath]
	mu.Unlock()
	if stillThere {
		t.Fatal("object should be deleted from S3 after DeleteBlob")
	}
}

func TestApplyFileStorageS3_Noop(t *testing.T) {
	// No file_storage section → no error, no store attached.
	if err := applyFileStorageS3(nil, nil); err != nil {
		t.Errorf("nil appCfg should be a no-op, got %v", err)
	}
	if err := applyFileStorageS3(nil, &project.AppConfig{}); err != nil {
		t.Errorf("empty appCfg should be a no-op, got %v", err)
	}
}

func TestApplyFileStorageS3_InvalidConfig(t *testing.T) {
	// file_storage.s3 present but missing bucket → surfaced at startup.
	appCfg := &project.AppConfig{FileStorage: &project.FileStorageConfig{S3: &project.S3Config{
		Endpoint: "s3.example.com", AccessKey: "k", SecretKey: "s",
	}}}
	if err := applyFileStorageS3(nil, appCfg); err == nil {
		t.Fatal("expected error for S3 config missing bucket")
	}
}
