package launcher

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfiguratorSaveFields_RejectsInvalidFieldName(t *testing.T) {
	h, cfgDir := newFileBaseHandler(t)
	h.runner = NewRunner()
	if err := os.MkdirAll(filepath.Join(cfgDir, "catalogs"), 0o755); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(cfgDir, "catalogs", "клиенты.yaml")
	initial := `name: Клиенты
fields:
  - name: Наименование
    type: string
`
	if err := os.WriteFile(yamlPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("entity", "Клиенты")
	form.Set("entity_kind", "Справочник")
	form.Set("field.0.name", "Наименование")
	form.Set("field.0.type", "string")
	form.Set("new_field.1.name", "Приход  От Клиента")
	form.Set("new_field.1.type", "number")

	rec := saveFieldsForm(t, h, "test", form)
	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON response: %v\n%s", err, rec.Body.String())
	}
	if resp.OK {
		t.Fatalf("сохранение невалидного имени прошло успешно: %s", rec.Body.String())
	}
	if !strings.Contains(resp.Error, "Приход  От Клиента") || !strings.Contains(resp.Error, "без пробелов") {
		t.Fatalf("ошибка не объясняет проблему имени: %q", resp.Error)
	}

	out, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("чтение YAML: %v", err)
	}
	if strings.Contains(string(out), "Приход  От Клиента") {
		t.Fatalf("невалидное поле было записано в YAML:\n%s", out)
	}
}

func TestConfiguratorSaveFields_AllowsSparseIndicesAfterRowDelete(t *testing.T) {
	h, cfgDir := newFileBaseHandler(t)
	h.runner = NewRunner()
	if err := os.MkdirAll(filepath.Join(cfgDir, "catalogs"), 0o755); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(cfgDir, "catalogs", "клиенты.yaml")
	initial := `name: Клиенты
fields:
  - name: Код
    type: string
  - name: СтароеПоле
    type: string
  - name: Наименование
    type: string
`
	if err := os.WriteFile(yamlPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("entity", "Клиенты")
	form.Set("entity_kind", "Справочник")
	form.Set("field.0.name", "Код")
	form.Set("field.0.type", "string")
	// field.1 отсутствует: это результат удаления средней строки в DOM.
	form.Set("field.2.name", "Наименование")
	form.Set("field.2.type", "string")

	rec := saveFieldsForm(t, h, "test", form)
	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON response: %v\n%s", err, rec.Body.String())
	}
	if !resp.OK {
		t.Fatalf("сохранение с разреженными индексами не прошло: %q", resp.Error)
	}

	out, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("чтение YAML: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "СтароеПоле") {
		t.Fatalf("удалённое поле осталось в YAML:\n%s", got)
	}
	if !strings.Contains(got, "name: Код") || !strings.Contains(got, "name: Наименование") {
		t.Fatalf("последующие поля потерялись после удаления средней строки:\n%s", got)
	}
}
