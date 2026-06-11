package gengen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir(t *testing.T) {
	// Create a temporary source directory with files
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "config"), 0o755)
	os.WriteFile(filepath.Join(src, "config", "app.yaml"), []byte("name: test\n"), 0o644)
	os.WriteFile(filepath.Join(src, "README.md"), []byte("# test\n"), 0o644)

	dst := t.TempDir()

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir() error: %v", err)
	}

	// Verify files were copied
	if _, err := os.Stat(filepath.Join(dst, "config", "app.yaml")); err != nil {
		t.Error("config/app.yaml was not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "README.md")); err != nil {
		t.Error("README.md was not copied")
	}
}

func TestCopyDir_NoOverwrite(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "existing.txt"), []byte("original\n"), 0o644)

	dst := t.TempDir()
	existingContent := []byte("do not overwrite\n")
	os.WriteFile(filepath.Join(dst, "existing.txt"), existingContent, 0o644)

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dst, "existing.txt"))
	if string(data) != "do not overwrite\n" {
		t.Error("existing file was overwritten!")
	}
}

func TestSanitizePrompt(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"тексты и переводы", "тексты-и-переводы"},
		{"оптовые продажи товаров", "оптовые-продажи-товаров"},
		{"очень длинный промпт который обрезается", "очень-длинный-промпт"},
		{"hello world", "hello-world"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizePrompt(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizePrompt(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerator_Generate_NoTemplate(t *testing.T) {
	g := &Generator{OutputDir: t.TempDir()}
	err := g.Generate("", nil)
	if err == nil {
		t.Error("expected error for empty template")
	}
}

func TestGenerator_Generate_WithTemplate(t *testing.T) {
	// Create a mock template
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "config"), 0o755)
	os.WriteFile(filepath.Join(src, "config", "app.yaml"), []byte("name: test\n"), 0o644)

	dst := t.TempDir()
	g := &Generator{OutputDir: dst}

	if err := g.Generate(src, nil); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "config", "app.yaml")); err != nil {
		t.Error("template was not generated")
	}
}
