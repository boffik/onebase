package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnum(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "e.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadEnum_NewFormatWithTitles(t *testing.T) {
	e, err := LoadEnumFile(writeEnum(t, `name: Приоритет
values:
  - name: Высокий
    titles:
      en: High
      de: Hoch
  - name: Низкий
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Values) != 2 || e.Values[0] != "Высокий" || e.Values[1] != "Низкий" {
		t.Fatalf("Values = %v", e.Values)
	}
	if e.ValueTitle("Высокий", "en") != "High" {
		t.Errorf("en = %q", e.ValueTitle("Высокий", "en"))
	}
	if e.ValueTitle("Высокий", "de") != "Hoch" {
		t.Errorf("de = %q", e.ValueTitle("Высокий", "de"))
	}
	if e.ValueTitle("Низкий", "en") != "Низкий" {
		t.Errorf("fallback = %q", e.ValueTitle("Низкий", "en"))
	}
	if e.ValueTitle("Высокий", "") != "Высокий" {
		t.Errorf("empty lang = %q", e.ValueTitle("Высокий", ""))
	}
}

func TestLoadEnum_OldFormatStillWorks(t *testing.T) {
	e, err := LoadEnumFile(writeEnum(t, `name: Статус
values: [Открыт, Закрыт]
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Values) != 2 || e.Values[0] != "Открыт" {
		t.Fatalf("Values = %v", e.Values)
	}
	if e.ValueTitle("Открыт", "en") != "Открыт" {
		t.Errorf("old format value should fall back to name, got %q", e.ValueTitle("Открыт", "en"))
	}
}
