package gengen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeGenerator_CreateNewCatalog(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "catalogs"), 0o755)

	delta := &DeltaManifest{
		NewCatalogs: []EntitySpec{
			{
				Name: "Контрагент",
				Fields: []FieldSpec{
					{Name: "Наименование", Type: "string"},
					{Name: "ИНН", Type: "string"},
				},
			},
		},
		NewFields:     make(map[string][]FieldSpec),
		NewTableParts: make(map[string][]TablePartSpec),
		NewDSLFiles:   make(map[string]string),
	}

	mg := &MergeGenerator{ProjectDir: dir}
	if err := mg.Merge(delta); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	path := filepath.Join(dir, "catalogs", "контрагент.yaml")
	if !fileExists(path) {
		t.Fatal("expected контрагент.yaml to be created")
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if len(content) == 0 {
		t.Error("expected non-empty YAML file")
	}
}

func TestMergeGenerator_CreateNewDocument(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "documents"), 0o755)

	delta := &DeltaManifest{
		NewDocuments: []EntitySpec{
			{
				Name: "РеализацияТоваров",
				Fields: []FieldSpec{
					{Name: "Контрагент", Type: "reference:Контрагент"},
					{Name: "Дата", Type: "date"},
				},
				TableParts: []TablePartSpec{
					{
						Name: "Товары",
						Fields: []FieldSpec{
							{Name: "Номенклатура", Type: "reference:Номенклатура"},
							{Name: "Количество", Type: "number"},
						},
					},
				},
			},
		},
		NewFields:     make(map[string][]FieldSpec),
		NewTableParts: make(map[string][]TablePartSpec),
		NewDSLFiles:   make(map[string]string),
	}

	mg := &MergeGenerator{ProjectDir: dir}
	if err := mg.Merge(delta); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	path := filepath.Join(dir, "documents", "реализациятоваров.yaml")
	if !fileExists(path) {
		t.Fatal("expected реализациятоваров.yaml to be created")
	}
}

func TestMergeGenerator_AddFieldsToExisting(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "catalogs"), 0o755)

	// Create existing catalog
	existingYAML := `name: Контрагент
fields:
  - name: Наименование
    type: string
  - name: ИНН
    type: string
`
	os.WriteFile(filepath.Join(dir, "catalogs", "контрагент.yaml"), []byte(existingYAML), 0o644)

	delta := &DeltaManifest{
		NewFields: map[string][]FieldSpec{
			"Контрагент": {
				{Name: "КПП", Type: "string"},
				{Name: "ЮрАдрес", Type: "string"},
			},
		},
		NewTableParts: make(map[string][]TablePartSpec),
		NewDSLFiles:   make(map[string]string),
	}

	mg := &MergeGenerator{ProjectDir: dir}
	if err := mg.Merge(delta); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	// Verify the file was modified
	data, _ := os.ReadFile(filepath.Join(dir, "catalogs", "контрагент.yaml"))
	content := string(data)

	// Check that new fields are present
	if !containsStr(content, "КПП") {
		t.Error("expected КПП field in modified YAML")
	}
	if !containsStr(content, "ЮрАдрес") {
		t.Error("expected ЮрАдрес field in modified YAML")
	}
	// Check that existing fields are preserved
	if !containsStr(content, "Наименование") {
		t.Error("expected Наименование field to be preserved")
	}
	if !containsStr(content, "ИНН") {
		t.Error("expected ИНН field to be preserved")
	}
}

func TestMergeGenerator_CreateEnum(t *testing.T) {
	dir := t.TempDir()

	delta := &DeltaManifest{
		NewEnums: []EnumSpec{
			{Name: "СтавкиНДС", Values: []string{"БезНДС", "0%", "10%", "20%"}},
		},
		NewFields:     make(map[string][]FieldSpec),
		NewTableParts: make(map[string][]TablePartSpec),
		NewDSLFiles:   make(map[string]string),
	}

	mg := &MergeGenerator{ProjectDir: dir}
	if err := mg.Merge(delta); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	path := filepath.Join(dir, "enums", "ставкиндс.yaml")
	if !fileExists(path) {
		t.Fatal("expected ставкиндс.yaml to be created")
	}
}

func TestMergeGenerator_CreateDSLFile(t *testing.T) {
	dir := t.TempDir()

	delta := &DeltaManifest{
		NewDSLFiles: map[string]string{
			"отчёт_продажи.os": "Процедура Сформировать()\n  Сообщить(\"Готово\")\nКонецПроцедуры\n",
		},
		NewFields:     make(map[string][]FieldSpec),
		NewTableParts: make(map[string][]TablePartSpec),
	}

	mg := &MergeGenerator{ProjectDir: dir}
	if err := mg.Merge(delta); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	path := filepath.Join(dir, "src", "отчёт_продажи.os")
	if !fileExists(path) {
		t.Fatal("expected DSL file to be created")
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !containsStr(content, "Сформировать") {
		t.Error("expected DSL content to contain Сформировать")
	}
}

func TestMergeGenerator_NoChanges(t *testing.T) {
	dir := t.TempDir()

	delta := &DeltaManifest{
		NewFields:     make(map[string][]FieldSpec),
		NewTableParts: make(map[string][]TablePartSpec),
		NewDSLFiles:   make(map[string]string),
	}

	mg := &MergeGenerator{ProjectDir: dir}
	if err := mg.Merge(delta); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || (len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
