package ui

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/boffik/onebase/internal/metadata"
)

// Внешний вид любого элемента управляемой формы обязан задаваться таблицей
// стилей, а не inline-атрибутом style: иначе ни вид формы (ui.style), ни своё
// оформление не могут его переопределить — inline сильнее любого селектора, и
// теме приходится бороться с ним через !important.
//
// В style= остаётся только то, что НЕ оформление:
//   - раскладка из YAML (width/height/halign/valign, flex/margin под неё);
//   - фон и цвет из YAML (background, title_color, header_background) — это
//     данные формы, а не вид интерфейса;
//   - состояние (display:none у скрытого hidden_when и у неактивной закладки).
var decorativeInlineProps = []string{
	"border", "border-radius", "box-shadow",
	"padding", "margin:", "margin-top", "margin-bottom",
	"font", "color", "background", "text-decoration", "line-height",
}

var inlineStyleAttrRe = regexp.MustCompile(`style="([^"]*)"`)

func TestManagedFormAppearanceComesFromStyleTable(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	form := &metadata.FormModule{
		Name: "ФормаОбъекта", Kind: "object", EntityName: "Заказ",
		LayoutKind: metadata.FormLayoutManaged,
		Elements: []*metadata.FormElement{
			{
				Kind: metadata.FormElementGroupBox, Name: "Реквизиты",
				TitleMap: map[string]string{"ru": "Реквизиты"}, Bordered: boolPtr(true),
				Children: []*metadata.FormElement{
					{Kind: metadata.FormElementField, Name: "ПолеНомер", DataPath: "Объект.Номер", Hint: "Подсказка"},
					{Kind: metadata.FormElementField, Name: "ПолеДата", DataPath: "Объект.Дата"},
					{Kind: metadata.FormElementCheckbox, Name: "ПолеГалка", DataPath: "Объект.Галка"},
					{Kind: metadata.FormElementLabel, Name: "Надпись", TitleMap: map[string]string{"ru": "Надпись"}},
					{Kind: metadata.FormElementButton, Name: "Кнопка", TitleMap: map[string]string{"ru": "Кнопка"}},
				},
			},
			{
				Kind: metadata.FormElementPages, Name: "Страницы",
				Children: []*metadata.FormElement{
					{
						Kind: metadata.FormElementPage, Name: "Первая",
						TitleMap: map[string]string{"ru": "Первая"},
						Children: []*metadata.FormElement{
							{Kind: metadata.FormElementField, Name: "ПолеКомментарий", DataPath: "Объект.Комментарий"},
						},
					},
					{Kind: metadata.FormElementPage, Name: "Вторая", TitleMap: map[string]string{"ru": "Вторая"}},
				},
			},
		},
	}
	entity := &metadata.Entity{
		Name: "Заказ", Kind: metadata.KindDocument,
		Fields: []metadata.Field{
			{Name: "Номер", Type: metadata.FieldTypeString},
			{Name: "Дата", Type: metadata.FieldTypeDate},
			{Name: "Галка", Type: metadata.FieldTypeBool},
			{Name: "Комментарий", Type: metadata.FieldTypeString},
		},
		Forms: []*metadata.FormModule{form},
	}

	body := renderManagedForm(t, entity, form, map[string]string{})

	// Область — карточка управляемой формы: общий каркас (шапка, навигация,
	// подвал страницы) живёт в других шаблонах и в этот контракт не входит.
	cardStart := strings.Index(body, `<div class="card ob-form-card">`)
	if cardStart < 0 {
		t.Fatal("карточка управляемой формы не найдена")
	}
	cardEnd := strings.Index(body[cardStart:], "</form>")
	if cardEnd < 0 {
		t.Fatal("форма управляемой карточки не закрыта")
	}
	card := body[cardStart : cardStart+cardEnd]

	for _, match := range inlineStyleAttrRe.FindAllStringSubmatch(card, -1) {
		style := html.UnescapeString(match[1])
		for _, prop := range decorativeInlineProps {
			if strings.Contains(style, prop) {
				t.Errorf("оформление задано inline (%q) — оно должно быть в таблице стилей: style=%q", prop, style)
			}
		}
	}

	// Сами правила оформления при этом обязаны быть в стиле вида.
	for _, rule := range []string{
		"fieldset.form-group-box{border:1px solid #e2e8f0;border-radius:8px;padding:12px 14px",
		".form-group-box>legend{font-weight:600;color:#475569;padding:0 6px;font-size:13px}",
		".managed-tabs{border:1px solid #e2e8f0;border-radius:8px;padding:12px 14px;margin-bottom:14px}",
		".managed-tab-headers{display:flex;gap:2px;border-bottom:2px solid #e2e8f0;margin-bottom:12px}",
		".managed-control-row{display:flex;gap:6px;align-items:center}",
		".form-decoration{padding:6px 0;color:#475569;font-size:13px}",
	} {
		if !strings.Contains(body, rule) {
			t.Errorf("в таблице стилей нет правила %q", rule)
		}
	}
}

// Ключ bordered: false по-прежнему снимает рамку — теперь классом, а не
// отсутствием inline-стиля.
func TestManagedFormBorderedOffStaysClassBased(t *testing.T) {
	off := false
	form := &metadata.FormModule{
		Name: "ФормаОбъекта", Kind: "object", EntityName: "Заказ",
		LayoutKind: metadata.FormLayoutManaged,
		Elements: []*metadata.FormElement{{
			Kind: metadata.FormElementGroupBox, Name: "Реквизиты", Bordered: &off,
			Children: []*metadata.FormElement{
				{Kind: metadata.FormElementField, Name: "ПолеНомер", DataPath: "Объект.Номер"},
			},
		}},
	}
	entity := &metadata.Entity{
		Name: "Заказ", Kind: metadata.KindDocument,
		Fields: []metadata.Field{{Name: "Номер", Type: metadata.FieldTypeString}},
		Forms:  []*metadata.FormModule{form},
	}

	body := renderManagedForm(t, entity, form, map[string]string{})
	if !strings.Contains(body, "fieldset.form-group-box.ob-group-plain{border:0;border-radius:0;padding:0}") {
		t.Error("рамку группы снимает не правило .ob-group-plain")
	}
}
