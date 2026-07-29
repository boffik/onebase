package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestReportExplainSampleSQLiteWithParams — регрессия на issue #473:
// `report explain --sample N` с переданными значениями параметров на SQLite
// падал «missing named argument "1::text"», потому что запрос компилировался
// без диалекта открытой БД (генерились Postgres-плейсхолдеры $N::text).
func TestReportExplainSampleSQLiteWithParams(t *testing.T) {
	projectDir := t.TempDir()
	writeProcrunFixture(t, projectDir, "config/app.yaml", "name: explain-test\nversion: \"1.0\"\n")
	writeProcrunFixture(t, projectDir, "catalogs/Товар.yaml", "name: Товар\nfields:\n  - name: Наименование\n    type: string\n")
	writeProcrunFixture(t, projectDir, "reports/ПоТовару.yaml", `name: ПоТовару
title: По товару
params:
  - name: Имя
    type: string
    label: "Имя"
query: |
  ВЫБРАТЬ Наименование
  ИЗ Справочник.Товар
  ГДЕ (&Имя ЕСТЬ ПУСТО ИЛИ Наименование = &Имя)
`)

	dbPath := filepath.Join(t.TempDir(), "explain.db")

	// Схема БД.
	migrate := &cobra.Command{}
	addBaseFlags(migrate)
	if err := migrate.Flags().Set("project", projectDir); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Flags().Set("sqlite", dbPath); err != nil {
		t.Fatal(err)
	}
	if err := runMigrate(migrate, nil); err != nil {
		t.Fatalf("runMigrate: %v", err)
	}

	// report explain --sample с переданным значением параметра.
	cmd := &cobra.Command{}
	addBaseFlags(cmd)
	cmd.Flags().Int("sample", 0, "")
	cmd.Flags().String("params", "", "")
	cmd.Flags().Bool("json", false, "")
	for k, v := range map[string]string{
		"project": projectDir,
		"sqlite":  dbPath,
		"sample":  "5",
		"params":  `{"Имя":"Молоко"}`,
	} {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatal(err)
		}
	}

	out, err := captureStdout(t, func() error {
		return runReportExplain(cmd, []string{"ПоТовару"})
	})
	if err != nil {
		t.Fatalf("runReportExplain: %v", err)
	}

	if strings.Contains(out, "missing named argument") {
		t.Fatalf("исполнение --sample на SQLite снова упало:\n%s", out)
	}
	if strings.Contains(out, "$1::text") {
		t.Fatalf("SQL содержит Postgres-плейсхолдеры вместо SQLite-диалекта:\n%s", out)
	}
	if strings.Contains(out, "Ошибка:") {
		t.Fatalf("explain вернул ошибку:\n%s", out)
	}
}
