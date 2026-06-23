package configdb_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/configdb"
	"github.com/ivantit66/onebase/internal/storage"
)

func newSQLiteRepo(t *testing.T) (*configdb.Repo, *storage.DB, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "config.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	t.Cleanup(db.Close)
	repo := configdb.New(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return repo, db, ctx
}

func TestValidatePath(t *testing.T) {
	valid := []string{
		"tree_order.yaml",
		"config/app.yaml",
		"src/модуль.module.os",
		"forms/заказ/форма.form.yaml",
		"printforms/заказ/печать.layout.yaml",
	}
	for _, p := range valid {
		if err := configdb.ValidatePath(p); err != nil {
			t.Fatalf("ValidatePath(%q): %v", p, err)
		}
	}

	invalid := []string{
		"",
		"../evil.yaml",
		"config/../../evil.yaml",
		"/tmp/evil.yaml",
		`config\evil.yaml`,
		"config//app.yaml",
		"config/app.yaml/",
		"config/evil:name.yaml",
		"config/\x00evil.yaml",
	}
	for _, p := range invalid {
		if err := configdb.ValidatePath(p); err == nil {
			t.Fatalf("ValidatePath(%q) succeeded, want error", p)
		}
	}
}

func TestRepoRejectsUnsafePaths(t *testing.T) {
	repo, _, ctx := newSQLiteRepo(t)

	if err := repo.SaveFile(ctx, "../evil.yaml", []byte("x")); err == nil {
		t.Fatal("SaveFile accepted traversal path")
	}
	if err := repo.DeleteFile(ctx, "../evil.yaml"); err == nil {
		t.Fatal("DeleteFile accepted traversal path")
	}
	if _, _, err := repo.ReadFile(ctx, "../evil.yaml"); err == nil {
		t.Fatal("ReadFile accepted traversal path")
	}
}

func TestExportToDirRejectsStoredTraversalPath(t *testing.T) {
	repo, db, ctx := newSQLiteRepo(t)
	_, err := db.Exec(ctx, `INSERT INTO _onebase_config(path, content) VALUES (?, ?)`, "../evil.yaml", []byte("x"))
	if err != nil {
		t.Fatalf("insert unsafe path: %v", err)
	}

	dst := t.TempDir()
	if err := repo.ExportToDir(ctx, dst); err == nil {
		t.Fatal("ExportToDir accepted stored traversal path")
	}
	if _, err := os.Stat(filepath.Join(dst, "..", "evil.yaml")); !os.IsNotExist(err) {
		t.Fatalf("ExportToDir wrote outside target, stat err=%v", err)
	}
}
