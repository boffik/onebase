package gengen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerator_Generate_TextsTemplate(t *testing.T) {
	// This test verifies the texts template generates correctly.
	// It uses the real templates/texts/ directory.
	templateDir := "../../templates/texts"
	if !dirExists(templateDir) {
		t.Skip("templates/texts/ not found (run from repo root)")
	}

	dst := t.TempDir()
	g := &Generator{OutputDir: dst}

	if err := g.Generate(templateDir, nil); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Verify expected files
	expected := []string{
		"config/app.yaml",
		"catalogs/события.yaml",
		"documents/текст.yaml",
		"documents/перевод_текста.yaml",
	}
	for _, f := range expected {
		path := filepath.Join(dst, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s not found", f)
		}
	}
}

func TestAnalyze_TextsTemplate(t *testing.T) {
	r := Analyze("тексты и переводы")
	if r.Domain != "texts" {
		t.Fatalf("Analyze() = %q, want texts", r.Domain)
	}
	if r.Template == "" {
		t.Fatal("Analyze() returned empty template")
	}
	if !dirExists(r.Template) {
		t.Fatalf("template dir %q does not exist", r.Template)
	}
}
