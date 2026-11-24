package ui

// Нативный контрол показывает дату по локали браузера. ПолеДаты по умолчанию
// редактирует календарный день, но date_mode может включить и ввод времени.

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/boffik/onebase/internal/metadata"
	"github.com/google/uuid"
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

func TestManagedForm_DateModeOverridesDatePicker(t *testing.T) {
	out := renderManagedElement(t, &metadata.FormElement{
		Kind: metadata.FormElementDatePicker, Name: "ДатаПоступления",
		DataPath: "Объект.ДатаПоступления", DateMode: metadata.FormDateModeDateTime,
	})
	if !strings.Contains(out, `type="datetime-local" step="1"`) ||
		!strings.Contains(out, `value="2026-06-20T10:00:00"`) {
		t.Fatalf("вариант «Дата и время» не применился к ПолеДаты:\n%s", out)
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

func TestManagedForm_DateModeDateOnly_PublicFormRoundTrip(t *testing.T) {
	ent := &metadata.Entity{
		Name: "ЗаписьСДатой", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Дата", Type: metadata.FieldTypeDate}},
		Forms: []*metadata.FormModule{managedObjectForm(&metadata.FormElement{
			Kind: metadata.FormElementField, Name: "ПолеДата", DataPath: "Объект.Дата",
			DateMode: metadata.FormDateModeDate,
		})},
	}
	srv, ctx := newSubmitTestServer(t, []*metadata.Entity{ent})
	id := uuid.New()
	initial := time.Date(2026, 6, 20, 10, 0, 0, 0, time.Local)
	if err := srv.store.Upsert(ctx, ent.Name, id, map[string]any{"Дата": initial}, ent); err != nil {
		t.Fatal(err)
	}
	page := открытьФормуОбъекта(t, srv, ent, id)
	if !strings.Contains(page, `<input type="date" name="Дата" value="2026-06-20"`) {
		t.Fatalf("управляемая форма не показывает календарную дату:\n%s", page)
	}

	записатьЗаявку(t, srv, ent, id, url.Values{"Дата": {"2026-06-21"}})
	stored, err := srv.store.GetByID(ctx, ent.Name, id, ent)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := stored["Дата"].(time.Time)
	if !ok {
		t.Fatalf("дата сохранена как %T, ожидался time.Time", stored["Дата"])
	}
	if got.In(time.Local).Format("2006-01-02T15:04:05") != "2026-06-21T00:00:00" {
		t.Fatalf("сохранённое значение = %s, дата должна сохраняться на местную полночь", got.In(time.Local))
	}
}
