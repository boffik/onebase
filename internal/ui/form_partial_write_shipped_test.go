package ui

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/project"
)

// Проверка на НАСТОЯЩЕЙ поставляемой конфигурации examples/crm: КонтактноеЛицо
// имеет управляемую форму, которая не размещает поле Наименование, — именно там
// «открыть карточку и нажать Записать» затирало представление справочника.
func TestShipped_CRM_ContactPersonKeepsName(t *testing.T) {
	p, err := project.Load("../../examples/crm")
	if err != nil {
		t.Skipf("проект не загрузился: %v", err)
	}
	defer p.Close()

	var ent *metadata.Entity
	for _, e := range p.Entities {
		if e.Name == "КонтактноеЛицо" {
			ent = e
			break
		}
	}
	if ent == nil {
		t.Skip("сущность КонтактноеЛицо не найдена")
	}
	form := pickManagedForm(ent, "object")
	if form == nil {
		t.Skip("у КонтактноеЛицо нет управляемой формы объекта")
	}

	// Собираем имена полей шапки, которые форма реально отрисует.
	placed := map[string]bool{}
	var walk func(el *metadata.FormElement)
	walk = func(el *metadata.FormElement) {
		if el == nil {
			return
		}
		if el.DataPath != "" && el.Kind != metadata.FormElementType("ТабличнаяЧасть") {
			placed[dpFieldName(el.DataPath)] = true
		}
		for _, c := range el.Children {
			walk(c)
		}
	}
	for _, el := range form.Elements {
		walk(el)
	}
	if placed["Наименование"] {
		t.Skip("форма стала показывать Наименование — сценарий неактуален")
	}
	t.Logf("форма НЕ размещает Наименование — это и есть дефект поставки")

	s, ctx := newSubmitTestServer(t, p.Entities)
	id := uuid.New()
	seed := map[string]any{"Наименование": "Иванов Иван Иванович"}
	if err := s.store.Upsert(ctx, ent.Name, id, seed, ent); err != nil {
		t.Fatal(err)
	}

	// POST ровно тех полей, что форма отрисовала бы (значения пустые — как если
	// бы пользователь ничего не заполнял и просто нажал «Записать»).
	form2 := url.Values{}
	for name := range placed {
		if _, ok := entityFieldByName(ent, name); ok {
			form2.Set(name, "")
		}
	}
	r := reqWithChi("POST", "/ui/catalog/КонтактноеЛицо/"+id.String(), form2,
		map[string]string{"entity": "КонтактноеЛицо", "id": id.String()})
	s.submitEdit(httptest.NewRecorder(), r)

	row, err := s.store.GetByID(ctx, ent.Name, id, ent)
	if err != nil {
		t.Fatal(err)
	}
	if row["Наименование"] != "Иванов Иван Иванович" {
		t.Fatalf("представление справочника затёрто: %v", row["Наименование"])
	}
	t.Log("Наименование сохранено")
}
