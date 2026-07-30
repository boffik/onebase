package backup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/project"
)

// fakeStore is an in-memory ObjectStore for exercising the S3 upload/rotation
// branch of createAutoBackup without touching the network.
type fakeStore struct {
	puts    map[string][]byte
	deleted []string
	putErr  error
}

func newFakeStore() *fakeStore { return &fakeStore{puts: map[string][]byte{}} }

func (f *fakeStore) PutObject(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	if f.putErr != nil {
		return f.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.puts[key] = b
	return nil
}

func (f *fakeStore) ListKeys(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range f.puts {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (f *fakeStore) DeleteObject(_ context.Context, key string) error {
	delete(f.puts, key)
	f.deleted = append(f.deleted, key)
	return nil
}

func staticDumper(name, content string) autoDumper {
	return func(_ context.Context, _ AutoTarget, outDir string) (string, error) {
		path := filepath.Join(outDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", err
		}
		return path, nil
	}
}

func TestCreateAutoBackup_UploadsToS3(t *testing.T) {
	dir := t.TempDir()
	cfg := &project.BackupConfig{
		Directory: dir,
		S3:        &project.S3Config{Bucket: "b", Prefix: "prod/"},
	}
	store := newFakeStore()

	_, err := createAutoBackup(context.Background(), cfg, AutoTarget{},
		staticDumper("backup_new.db", "payload"),
		func(*project.S3Config) (ObjectStore, error) { return store, nil })
	if err != nil {
		t.Fatalf("createAutoBackup: %v", err)
	}
	got, ok := store.puts["prod/backup_new.db"]
	if !ok {
		t.Fatalf("expected object prod/backup_new.db, have keys %v", keysOf(store))
	}
	if string(got) != "payload" {
		t.Errorf("uploaded content = %q, want payload", got)
	}
}

func TestCreateAutoBackup_RotatesS3(t *testing.T) {
	dir := t.TempDir()
	cfg := &project.BackupConfig{
		Directory: dir,
		S3:        &project.S3Config{Bucket: "b", Prefix: "prod/", KeepLast: 2},
	}
	store := newFakeStore()
	// Pre-existing objects (older) plus a non-backup file that must be ignored.
	store.puts["prod/backup_2026-01-01.db"] = []byte("1")
	store.puts["prod/backup_2026-01-02.db"] = []byte("2")
	store.puts["prod/backup_2026-01-03.db"] = []byte("3")
	store.puts["prod/notes.txt"] = []byte("keep me")

	_, err := createAutoBackup(context.Background(), cfg, AutoTarget{},
		staticDumper("backup_2026-01-04.db", "4"),
		func(*project.S3Config) (ObjectStore, error) { return store, nil })
	if err != nil {
		t.Fatalf("createAutoBackup: %v", err)
	}

	// KeepLast=2 → newest two (04, 03) remain; 01 and 02 deleted; notes.txt kept.
	for _, want := range []string{"prod/backup_2026-01-04.db", "prod/backup_2026-01-03.db", "prod/notes.txt"} {
		if _, ok := store.puts[want]; !ok {
			t.Errorf("expected %s to survive rotation; keys=%v", want, keysOf(store))
		}
	}
	for _, gone := range []string{"prod/backup_2026-01-01.db", "prod/backup_2026-01-02.db"} {
		if _, ok := store.puts[gone]; ok {
			t.Errorf("expected %s to be rotated away", gone)
		}
	}
}

func TestCreateAutoBackup_S3ErrorKeepsLocal(t *testing.T) {
	dir := t.TempDir()
	cfg := &project.BackupConfig{
		Directory: dir,
		S3:        &project.S3Config{Bucket: "b", Prefix: "prod/"},
	}
	store := newFakeStore()
	store.putErr = errors.New("network down")

	path, err := createAutoBackup(context.Background(), cfg, AutoTarget{},
		staticDumper("backup_new.db", "payload"),
		func(*project.S3Config) (ObjectStore, error) { return store, nil })
	if err == nil || !strings.Contains(err.Error(), "s3 upload") {
		t.Fatalf("expected s3 upload error, got %v", err)
	}
	// Local backup must remain despite the S3 failure.
	if filepath.Base(path) != "backup_new.db" {
		t.Fatalf("path = %s", path)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("local backup should survive S3 failure: %v", statErr)
	}
}

func TestS3Key(t *testing.T) {
	cases := map[string][2]string{
		"prod/backup_x.db": {"prod/", "backup_x.db"},
		"prod/backup_y.db": {"prod", "backup_y.db"},
		"backup_z.db":      {"", "backup_z.db"},
	}
	for want, in := range cases {
		if got := s3Key(in[0], in[1]); got != want {
			t.Errorf("s3Key(%q,%q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func keysOf(f *fakeStore) []string {
	var ks []string
	for k := range f.puts {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
