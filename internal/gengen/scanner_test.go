package gengen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanProjectFromFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	manifest, err := ScanProjectFromFiles(dir)
	if err != nil {
		t.Fatalf("ScanProjectFromFiles() error: %v", err)
	}
	if len(manifest.Catalogs) != 0 {
		t.Error("expected no catalogs")
	}
	if len(manifest.Documents) != 0 {
		t.Error("expected no documents")
	}
}

func TestScanProjectFromFiles_Catalogs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "catalogs"), 0o755)

	// Create a catalog YAML file
	yamlContent := `name: Контрагент
fields:
  - name: Наименование
    type: string
  - name: ИНН
    type: string
`
	os.WriteFile(filepath.Join(dir, "catalogs", "контрагент.yaml"), []byte(yamlContent), 0o644)

	manifest, err := ScanProjectFromFiles(dir)
	if err != nil {
		t.Fatalf("ScanProjectFromFiles() error: %v", err)
	}

	if _, ok := manifest.Catalogs["Контрагент"]; !ok {
		t.Fatal("expected Контрагент catalog")
	}

	cat := manifest.Catalogs["Контрагент"]
	if len(cat.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(cat.Fields))
	}
	if cat.Fields[0].Name != "Наименование" {
		t.Errorf("expected field 0 = Наименование, got %s", cat.Fields[0].Name)
	}
}

func TestScanProjectFromFiles_Documents(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "documents"), 0o755)

	yamlContent := `name: РеализацияТоваров
fields:
  - name: Контрагент
    type: reference:Контрагент
  - name: Дата
    type: date
tableparts:
  - name: Товары
    fields:
      - name: Номенклатура
        type: reference:Номенклатура
      - name: Количество
        type: number
`
	os.WriteFile(filepath.Join(dir, "documents", "реализация_товаров.yaml"), []byte(yamlContent), 0o644)

	manifest, err := ScanProjectFromFiles(dir)
	if err != nil {
		t.Fatalf("ScanProjectFromFiles() error: %v", err)
	}

	if _, ok := manifest.Documents["РеализацияТоваров"]; !ok {
		t.Fatal("expected РеализацияТоваров document")
	}

	doc := manifest.Documents["РеализацияТоваров"]
	if len(doc.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(doc.Fields))
	}
	if len(doc.TableParts) != 1 {
		t.Errorf("expected 1 table part, got %d", len(doc.TableParts))
	}
	if doc.TableParts[0].Name != "Товары" {
		t.Errorf("expected TP name = Товары, got %s", doc.TableParts[0].Name)
	}
}

func TestScanProjectFromFiles_Enums(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "enums"), 0o755)

	yamlContent := `name: СтавкиНДС
values:
  - БезНДС
  - "0%"
  - "10%"
  - "20%"
`
	os.WriteFile(filepath.Join(dir, "enums", "ставки_ндс.yaml"), []byte(yamlContent), 0o644)

	manifest, err := ScanProjectFromFiles(dir)
	if err != nil {
		t.Fatalf("ScanProjectFromFiles() error: %v", err)
	}

	if _, ok := manifest.Enums["СтавкиНДС"]; !ok {
		t.Fatal("expected СтавкиНДС enum")
	}

	enum := manifest.Enums["СтавкиНДС"]
	if len(enum.Values) != 4 {
		t.Errorf("expected 4 values, got %d", len(enum.Values))
	}
}

func TestScanProjectFromFiles_DSL(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)

	os.WriteFile(filepath.Join(dir, "src", "отчёт_продажи.os"), []byte("Процедура Сформировать()\nКонецПроцедуры\n"), 0o644)

	manifest, err := ScanProjectFromFiles(dir)
	if err != nil {
		t.Fatalf("ScanProjectFromFiles() error: %v", err)
	}

	if _, ok := manifest.DSLFiles["отчёт_продажи.os"]; !ok {
		t.Fatal("expected DSL file")
	}
}
