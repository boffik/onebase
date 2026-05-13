package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
)

func TestDumpRestoreSQLite(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "live.db")
	backupDir := filepath.Join(dir, "backups")

	// 1) Create live DB with one row.
	db, err := storage.ConnectSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	if _, err := db.Exec(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO t(name) VALUES('alpha')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	// 2) Dump (VACUUM INTO).
	outPath, err := DumpSQLite(ctx, dbPath, backupDir)
	if err != nil {
		t.Fatalf("DumpSQLite: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	// 3) Modify live DB.
	db, _ = storage.ConnectSQLite(ctx, dbPath)
	if _, err := db.Exec(ctx, "INSERT INTO t(name) VALUES('beta')"); err != nil {
		t.Fatalf("insert beta: %v", err)
	}
	var n int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM t").Scan(&n)
	if n != 2 {
		t.Fatalf("live before restore: count = %d, want 2", n)
	}
	db.Close()

	// 4) Restore — must replace file, dropping the second row.
	if err := RestoreSQLite(ctx, dbPath, outPath); err != nil {
		t.Fatalf("RestoreSQLite: %v", err)
	}
	db, _ = storage.ConnectSQLite(ctx, dbPath)
	defer db.Close()
	_ = db.QueryRow(ctx, "SELECT count(*) FROM t").Scan(&n)
	if n != 1 {
		t.Fatalf("after restore: count = %d, want 1", n)
	}
	var name string
	_ = db.QueryRow(ctx, "SELECT name FROM t").Scan(&name)
	if name != "alpha" {
		t.Fatalf("after restore: name = %q, want alpha", name)
	}
}
