package printform

import (
	"strings"
	"testing"
)

// richLayout — макет с одним параметром, привязанным к полю «Результат».
func richLayout() *LayoutTemplate {
	return &LayoutTemplate{
		Name:     "Карточка",
		Document: "Задача",
		Areas: []*LayoutArea{
			{
				Name: "Шапка",
				Rows: []LayoutRow{
					{Cells: []LayoutCell{
						{Text: "Результат:"},
						{Parameter: "Результат"},
					}},
				},
			},
		},
		Binding: &Binding{
			Sequence:   []string{"Шапка"},
			Parameters: map[string]string{"Результат": "Результат"},
		},
	}
}

// TestBuildSheetRichTextField — параметр, привязанный к richtext-полю сущности,
// кладёт санитизированный HTML в Cell.RichHTML (а не в Text); HTML-рендер выводит
// форматирование (теги не экранированы).
func TestBuildSheetRichTextField(t *testing.T) {
	ctx := &RenderContext{
		Document: map[string]any{
			"Результат": `<p><b>Готово</b></p><script>alert(1)</script>`,
		},
		RichTextFields: map[string]bool{"результат": true},
	}
	doc, err := BuildSheet(richLayout(), ctx)
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}

	// Находим ячейку с RichHTML.
	var rich string
	for _, cell := range doc.Cells {
		if cell != nil && cell.RichHTML != "" {
			rich = cell.RichHTML
		}
	}
	if rich == "" {
		t.Fatalf("ни одна ячейка не получила RichHTML")
	}
	// Санитайз применён: <b> сохранён, <script> вырезан.
	if !strings.Contains(rich, "<b>Готово</b>") {
		t.Errorf("форматирование потеряно: %q", rich)
	}
	if strings.Contains(strings.ToLower(rich), "<script") {
		t.Errorf("санитайз не применён, <script> остался: %q", rich)
	}

	// HTML-рендер выводит richtext как HTML-блок (теги не экранированы).
	html := doc.HTMLString()
	if !strings.Contains(html, "<b>Готово</b>") {
		t.Errorf("HTML не содержит форматирование richtext\n%s", html)
	}
	if strings.Contains(html, "&lt;b&gt;") {
		t.Errorf("richtext экранирован в HTML")
	}
}

// TestBuildSheetRichTextFieldDefenseInDepth — даже если значение пришло «грязным»
// (в обход санитайза на сохранении), BuildSheet санитизирует его при сборке.
func TestBuildSheetRichTextFieldDefenseInDepth(t *testing.T) {
	ctx := &RenderContext{
		Document: map[string]any{
			"Результат": `<img src="javascript:alert(1)"><b onclick="evil()">x</b>`,
		},
		RichTextFields: map[string]bool{"результат": true},
	}
	doc, _ := BuildSheet(richLayout(), ctx)
	// Проверяем сам санитизированный контент ячейки (а не весь документ — в тулбаре
	// есть легитимные onclick="history.back()" у кнопок).
	var rich string
	for _, cell := range doc.Cells {
		if cell != nil && cell.RichHTML != "" {
			rich = cell.RichHTML
		}
	}
	low := strings.ToLower(rich)
	if strings.Contains(low, "javascript:") {
		t.Errorf("javascript: src не вырезан: %s", rich)
	}
	if strings.Contains(low, "onclick") {
		t.Errorf("on*-атрибут не вырезан: %s", rich)
	}
}

// TestBuildSheetNonRichFieldStaysText — обычное поле (нет в RichTextFields) идёт
// в Text как раньше: экранируется, RichHTML пустой.
func TestBuildSheetNonRichFieldStaysText(t *testing.T) {
	ctx := &RenderContext{
		Document: map[string]any{
			"Результат": `<b>не richtext</b>`, // то же значение, но поле НЕ richtext
		},
		// RichTextFields пуст — поле трактуется как обычный текст.
	}
	doc, err := BuildSheet(richLayout(), ctx)
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	for _, cell := range doc.Cells {
		if cell != nil && cell.RichHTML != "" {
			t.Fatalf("обычное поле получило RichHTML: %q", cell.RichHTML)
		}
	}
	// Текст экранируется в HTML (как раньше).
	html := doc.HTMLString()
	if !strings.Contains(html, "&lt;b&gt;") {
		t.Errorf("обычный текст не экранирован\n%s", html)
	}
}

// TestIsRichTextField — детектор richtext-выражений.
func TestIsRichTextField(t *testing.T) {
	ctx := &RenderContext{RichTextFields: map[string]bool{"результат": true}}
	cases := []struct {
		expr string
		want bool
	}{
		{"Результат", true},
		{"результат", true},            // регистронезависимо
		{"Результат | number:2", true}, // формат отбрасывается
		{"Покупатель", false},          // не richtext-поле
		{"Поле.ПодПоле", false},        // ссылка — не richtext
		{"@row", false},                // спецвыражение
		{"Итог.Товары.Сумма", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ctx.isRichTextField(c.expr); got != c.want {
			t.Errorf("isRichTextField(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
	// nil-контекст / пустое множество → всегда false.
	var nilCtx *RenderContext
	if nilCtx.isRichTextField("Результат") {
		t.Errorf("nil-контекст вернул true")
	}
	empty := &RenderContext{}
	if empty.isRichTextField("Результат") {
		t.Errorf("пустое множество вернуло true")
	}
}
