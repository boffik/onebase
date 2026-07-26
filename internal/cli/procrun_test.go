package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
	"github.com/spf13/cobra"
)

func TestRunProcrunEnsuresAuditSchema(t *testing.T) {
	projectDir := t.TempDir()
	writeProcrunFixture(t, projectDir, "config/app.yaml", "name: procrun-test\nversion: \"1.0\"\n")
	writeProcrunFixture(t, projectDir, "processors/Проверка.yaml", "name: Проверка\n")
	writeProcrunFixture(t, projectDir, "src/Проверка.proc.os", `Процедура Выполнить()
    Сообщить("ok");
КонецПроцедуры
`)

	dbPath := filepath.Join(t.TempDir(), "procrun.db")
	cmd := &cobra.Command{}
	addBaseFlags(cmd)
	cmd.Flags().String("proc", "", "")
	cmd.Flags().StringArray("set", nil, "")
	cmd.Flags().StringArray("file", nil, "")
	if err := cmd.Flags().Set("project", projectDir); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("sqlite", dbPath); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("proc", "Проверка"); err != nil {
		t.Fatal(err)
	}

	if err := runProcrun(cmd, nil); err != nil {
		t.Fatalf("runProcrun: %v", err)
	}

	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM _audit").Scan(&count); err != nil {
		t.Fatalf("procrun did not initialize _audit: %v", err)
	}
}

func writeProcrunFixture(t *testing.T, root, relativePath, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
