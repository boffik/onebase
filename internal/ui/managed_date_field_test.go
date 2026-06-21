package ui

// Issue #150: формат даты в форме. Полноценный произвольный формат
// (dd.MM.yyyy HH:mm) для редактируемого поля невозможен — нативный HTML5
// контрол показывает дату по локали браузера и не настраивается. Но элемент
// kind: ПолеДаты (он уже есть в схеме — FormElementDatePicker) должен
// рендериться как нативный выбор ДАТЫ без времени (<input type="date">),
// который в ru-локали показывается как дд.ММ.гггг и корректно сохраняется
// (formToFields парсит "2006-01-02"). Раньше ПолеДаты не рендерился вовсе.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

func renderManagedElement(t *testing.T, el *metadata.FormElement) string {
	t.Helper()
	ent := &metadata.Entity{
		Name: "Письмо",
		Kind: metadata.KindDocument,
		Fields: []metadata.Field{
			{Name: "ДатаПоступления", Type: metadata.FieldTypeDate},
		},
	}
	ctx := map[string]any{
		"Entity":      ent,
		"Values":      map[string]string{"ДатаПоступления": "2026-06-20T10:00"},
		"RefOptions":  map[string]any{},
		"EnumOptions": map[string]any{},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "managed-element", map[string]any{"El": el, "Ctx": ctx}); err != nil {
		t.Fatalf("execute managed-element: %v", err)
	}
	return buf.String()
}

func TestManagedForm_PoleDatyRendersDateInput(t *testing.T) {
	out := renderManagedElement(t, &metadata.FormElement{
		Kind:     metadata.FormElementDatePicker,
		Name:     "ДатаПоступления",
		DataPath: "Объект.ДатаПоступления",
	})
	if !strings.Contains(out, `type="date"`) {
		t.Errorf("ПолеДаты должно рендерить <input type=\"date\">, получили:\n%s", out)
	}
	if strings.Contains(out, "datetime-local") {
		t.Errorf("ПолеДаты не должно рендерить datetime-local")
	}
	if !strings.Contains(out, `value="2026-06-20"`) {
		t.Errorf("значение ПолеДаты должно быть датой без времени (2026-06-20):\n%s", out)
	}
}

func TestManagedForm_PoleVvodaDateStaysDatetimeLocal(t *testing.T) {
	out := renderManagedElement(t, &metadata.FormElement{
		Kind:     metadata.FormElementField,
		Name:     "ДатаПоступления",
		DataPath: "Объект.ДатаПоступления",
	})
	if !strings.Contains(out, "datetime-local") {
		t.Errorf("ПолеВвода по дате должно остаться datetime-local:\n%s", out)
	}
}
