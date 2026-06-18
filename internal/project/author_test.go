package project

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig_Authorship проверяет, что author/copyright/license из app.yaml
// разбираются в AppConfig (план 69).
func TestLoadConfig_Authorship(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	appYAML := `name: Demo
version: "1.0"
author: Иван Титов
copyright: © 2026 ООО «Ромашка»
license: MIT
`
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), []byte(appYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Author != "Иван Титов" {
		t.Errorf("Author = %q, want %q", cfg.Author, "Иван Титов")
	}
	if cfg.Copyright != "© 2026 ООО «Ромашка»" {
		t.Errorf("Copyright = %q", cfg.Copyright)
	}
	if cfg.License != "MIT" {
		t.Errorf("License = %q, want MIT", cfg.License)
	}
}

// TestLoadConfig_AuthorshipOptional — поля необязательны: без них пусто, без ошибок.
func TestLoadConfig_AuthorshipOptional(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), []byte("name: Bare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Author != "" || cfg.Copyright != "" || cfg.License != "" {
		t.Errorf("ожидались пустые поля авторства, получено: %q/%q/%q", cfg.Author, cfg.Copyright, cfg.License)
	}
}
