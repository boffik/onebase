package printform

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDirStandaloneLayout verifies that a standalone *.layout.yaml (no paired
// .os) is loaded as a declarative LayoutForm with Document from the document field.
func TestLoadDirStandaloneLayout(t *testing.T) {
	dir := t.TempDir()
	const src = `name: Накладная
document: Реализация
areas:
  - name: Шапка
    rows:
      - cells:
          - text: "Заголовок"
binding:
  parameters:
    Номер: "Номер"
`
	if err := os.WriteFile(filepath.Join(dir, "накладная.layout.yaml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	forms, dslForms, layoutForms, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(forms) != 0 || len(dslForms) != 0 {
		t.Fatalf("unexpected yaml/dsl forms: %d/%d", len(forms), len(dslForms))
	}
	if len(layoutForms) != 1 {
		t.Fatalf("expected 1 layout form, got %d", len(layoutForms))
	}
	lf := layoutForms[0]
	if lf.Name != "накладная" || lf.Document != "Реализация" {
		t.Fatalf("layout form fields: name=%q doc=%q", lf.Name, lf.Document)
	}
	if lf.Layout == nil || lf.Layout.Area("Шапка") == nil {
		t.Fatalf("layout not parsed: %+v", lf.Layout)
	}
}

// TestLoadDirPairedLayoutNotStandalone ensures a .layout.yaml paired with a .os
// is attached to the DSL form, not surfaced as a standalone LayoutForm.
func TestLoadDirPairedLayoutNotStandalone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "форма.os"),
		[]byte("// Документ: Реализация\nПроцедура Сформировать() Экспорт\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "форма.layout.yaml"),
		[]byte("name: форма\nareas:\n  - name: A\n    rows:\n      - cells:\n          - text: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, dslForms, layoutForms, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(layoutForms) != 0 {
		t.Fatalf("paired layout must not be standalone, got %d", len(layoutForms))
	}
	if len(dslForms) != 1 || dslForms[0].Layout == nil {
		t.Fatalf("paired layout not attached to DSL form: %+v", dslForms)
	}
}

// TestLoadDirSubfolderStandaloneLayout: standalone layout in an entity subfolder
// takes Document from the folder name when document field is absent.
func TestLoadDirSubfolderStandaloneLayout(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "РеализацияТоваров")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "товарная.layout.yaml"),
		[]byte("name: товарная\nareas:\n  - name: A\n    rows:\n      - cells:\n          - text: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, layoutForms, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(layoutForms) != 1 || layoutForms[0].Document != "РеализацияТоваров" {
		t.Fatalf("subfolder layout document: %+v", layoutForms)
	}
}
