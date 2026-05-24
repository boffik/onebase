package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

// listManagedFormsFromFS должен возвращать nil/nil для проектов без forms/.
func TestListManagedFormsFromFS_NoDir(t *testing.T) {
	dir := t.TempDir()
	b := &Base{Path: dir, ConfigSource: "file"}
	forms, err := listManagedFormsFromFS(b)
	if err != nil {
		t.Fatalf("ошибка для отсутствующего forms/: %v", err)
	}
	if forms != nil {
		t.Errorf("ожидался nil, получено %d форм", len(forms))
	}
}

// listManagedFormsFromFS обходит forms/<entity>/*.form.yaml и подгружает
// соседний .form.os если он есть.
func TestListManagedFormsFromFS_TwoForms(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "forms", "контрагент"), 0o755); err != nil {
		t.Fatal(err)
	}
	yamlBody := `schema: onebase.form/v1
form:
  name: ФормаОбъекта
  kind: object
  entity: Контрагент
elements: []
`
	osBody := "// модуль формы"
	os.WriteFile(filepath.Join(dir, "forms", "контрагент", "объекта.form.yaml"), []byte(yamlBody), 0o644)
	os.WriteFile(filepath.Join(dir, "forms", "контрагент", "объекта.form.os"), []byte(osBody), 0o644)

	listYAML := `schema: onebase.form/v1
form:
  name: ФормаСписка
  kind: list
  entity: Контрагент
`
	os.WriteFile(filepath.Join(dir, "forms", "контрагент", "списка.form.yaml"), []byte(listYAML), 0o644)

	b := &Base{Path: dir, ConfigSource: "file"}
	forms, err := listManagedFormsFromFS(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 2 {
		t.Fatalf("ожидалось 2 формы, получили %d", len(forms))
	}

	// Найдём объектную форму, проверим наличие .form.os и kind.
	var obj *cfgManagedForm
	for i := range forms {
		if strings.EqualFold(forms[i].Name, "объекта") {
			obj = &forms[i]
			break
		}
	}
	if obj == nil {
		t.Fatal("форма «объекта» не найдена")
	}
	if !obj.HasOS {
		t.Error("HasOS должен быть true (есть .form.os)")
	}
	if obj.Kind != "object" {
		t.Errorf("kind = %q, ожидался object", obj.Kind)
	}
	if !strings.Contains(obj.YAML, "ФормаОбъекта") {
		t.Errorf("YAML не содержит имя формы: %q", obj.YAML)
	}
}

// extractFormKindFromYAML — точечный тест маленького helper'а.
func TestExtractFormKindFromYAML(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"form:\n  kind: object\n", "object"},
		{"form:\n  kind: list\n", "list"},
		{"form:\n  name: X\n", ""},
		{"  kind: choice\n", "choice"},
	}
	for _, c := range cases {
		got := extractFormKindFromYAML(c.in)
		if got != c.want {
			t.Errorf("extractFormKindFromYAML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// renderManagedFormPreview должен вернуть HTML с заголовком managed-маркером
// и отрисовать переданные элементы.
func TestRenderManagedFormPreview(t *testing.T) {
	fm := &metadata.FormModule{
		EntityName: "Контрагент",
		Title:      map[string]string{"ru": "Карточка контрагента"},
		Elements: []*metadata.FormElement{
			{
				Kind:     metadata.FormElementGroupBox,
				Name:     "Реквизиты",
				TitleMap: map[string]string{"ru": "Реквизиты"},
				Children: []*metadata.FormElement{
					{
						Kind:     metadata.FormElementField,
						Name:     "ПолеНаименование",
						TitleMap: map[string]string{"ru": "Наименование"},
						DataPath: "Объект.Наименование",
						Required: true,
					},
					{
						Kind:     metadata.FormElementCheckbox,
						Name:     "ПолеАктивен",
						TitleMap: map[string]string{"ru": "Активен"},
						DataPath: "Объект.Активен",
					},
				},
			},
		},
	}
	html := renderManagedFormPreview(fm)
	must := []string{
		"Карточка контрагента",
		"◇ managed",
		"<legend>Реквизиты</legend>",
		"Наименование",
		"Активен",
		"<input type=\"checkbox\"",
		`class="req"`, // звёздочка для required-поля
	}
	for _, s := range must {
		if !strings.Contains(html, s) {
			t.Errorf("preview не содержит %q", s)
		}
	}
}

// previewErrorHTML экранирует сообщение и возвращает валидный HTML.
func TestPreviewErrorHTML(t *testing.T) {
	html := previewErrorHTML("parse yaml: line 5: bad <token>")
	if !strings.Contains(html, "Ошибка YAML") {
		t.Error("нет заголовка")
	}
	if !strings.Contains(html, "&lt;token&gt;") {
		t.Error("XML-токены не экранированы")
	}
}

// formFiles генерирует пути в lowercase.
func TestFormFiles(t *testing.T) {
	b := &Base{Path: "C:/proj"}
	yp, op := formFiles(b, "Контрагент", "ФормаОбъекта")
	wantYP := filepath.Join("C:/proj", "forms", "контрагент", "формаобъекта.form.yaml")
	wantOP := filepath.Join("C:/proj", "forms", "контрагент", "формаобъекта.form.os")
	if yp != wantYP {
		t.Errorf("yp = %q, want %q", yp, wantYP)
	}
	if op != wantOP {
		t.Errorf("op = %q, want %q", op, wantOP)
	}
}
