package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigMissingUsesProjectName(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != filepath.Base(dir) {
		t.Fatalf("name=%q, want %q", cfg.Name, filepath.Base(dir))
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	dir := writeAppConfig(t, "name: Demo\nlimtis:\n  report_max_rows: 10\n")
	_, err := LoadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "field limtis not found") {
		t.Fatalf("error=%v, want unknown-field diagnostic", err)
	}
}

func TestLoadConfigAcceptsLegacyIgnoredFields(t *testing.T) {
	dir := writeAppConfig(t, `name: DocFlow
attachments:
  storage_type: onebase_attachments
  storage_location: database:_attachments
  max_file_size_mb: 50
  allowed_types: [pdf, png]
  office_allowed_types: [doc, docx, xls, xlsx]
russian_post:
  enabled: true
  integration_mode: mock
`)
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("legacy app.yaml must remain loadable: %v", err)
	}
	if cfg.Attachments == nil {
		t.Fatal("Attachments is nil")
	}
	if cfg.Attachments.DeprecatedStorageType != "onebase_attachments" ||
		cfg.Attachments.DeprecatedStorageLocation != "database:_attachments" {
		t.Fatalf("legacy attachment keys were not decoded: %+v", cfg.Attachments)
	}
	if len(cfg.Attachments.DeprecatedOfficeAllowedTypes) != 4 {
		t.Fatalf("office_allowed_types = %v", cfg.Attachments.DeprecatedOfficeAllowedTypes)
	}
	if cfg.DeprecatedRussianPost["integration_mode"] != "mock" {
		t.Fatalf("russian_post = %#v", cfg.DeprecatedRussianPost)
	}
}

func TestLoadConfigRejectsMalformedYAML(t *testing.T) {
	dir := writeAppConfig(t, "name: [broken\n")
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("malformed app.yaml must fail")
	}
}

func TestLoadConfigRejectsMultipleDocuments(t *testing.T) {
	dir := writeAppConfig(t, "name: First\n---\nname: Second\n")
	if _, err := LoadConfig(dir); err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("error=%v, want multiple-document diagnostic", err)
	}
}

func TestLoadConfigReturnsNonNotExistReadErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config", "app.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("unreadable app.yaml path must fail")
	}
}

func writeAppConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
