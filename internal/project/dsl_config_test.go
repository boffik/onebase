package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_DSLStrictLexicalScope(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`name: Test
dsl:
  strict_lexical_scope: true
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.DSL == nil || !cfg.DSL.StrictLexicalScope {
		t.Fatalf("dsl.strict_lexical_scope разобран неверно: %+v", cfg.DSL)
	}
}
