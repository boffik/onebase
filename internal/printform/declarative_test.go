package printform

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/sheet"
)

// buildSheetCtx — синтетический контекст: документ с одной ТЧ.
func buildSheetCtx() *RenderContext {
	return &RenderContext{
		Document: map[string]any{
			"Номер":      "001",
			"Покупатель": "ref-buyer",
		},
		Refs: map[string]map[string]any{
			"ref-buyer": {"наименование": "ИП Иванов"},
			"ref-g1":    {"наименование": "Стол"},
			"ref-g2":    {"наименование": "Стул"},
		},
		TableParts: map[string][]map[string]any{
			"Товары": {
				{"Номенклатура": "ref-g1", "Количество": 2.0, "Сумма": 100.0},
				{"Номенклатура": "ref-g2", "Количество": 3.0, "Сумма": 250.0},
			},
		},
	}
}

// miniLayout — шапка (повтор) + строка-повтор по ТЧ + итог.
func miniLayout() *LayoutTemplate {
	return &LayoutTemplate{
		Name:     "Накладная",
		Document: "Реализация",
		Areas: []*LayoutArea{
			{
				Name: "Шапка",
				Rows: []LayoutRow{
					{Cells: []LayoutCell{
						{Text: "Накладная № {{Номер}}", ColSpan: 3, Align: "center", Bold: true},
					}},
					{Cells: []LayoutCell{
						{Text: "Покупатель:"},
						{Parameter: "Покупатель", ColSpan: 2},
					}},
				},
			},
			{
				Name: "ШапкаТаблицы",
				Rows: []LayoutRow{
					{Cells: []LayoutCell{
						{Text: "№"}, {Text: "Товар"}, {Text: "Сумма"},
					}},
				},
			},
			{
				Name: "Строка",
				Rows: []LayoutRow{
					{Cells: []LayoutCell{
						{Parameter: "Ном"},
						{Parameter: "Товар"},
						{Parameter: "Сум"},
					}},
				},
			},
			{
				Name: "Итог",
				Rows: []LayoutRow{
					{Cells: []LayoutCell{
						{Text: "Итого:", ColSpan: 2},
						{Parameter: "Всего"},
					}},
				},
			},
		},
		Binding: &Binding{
			Sequence:     []string{"Шапка", "ШапкаТаблицы", "Строка", "Итог"},
			RepeatHeader: "ШапкаТаблицы",
			Parameters: map[string]string{
				"Покупатель": "Покупатель",
				"Всего":      "Итог.Товары.Сумма | number:2",
			},
			Repeat: []RepeatBinding{
				{
					Area:   "Строка",
					Source: "Товары",
					Parameters: map[string]string{
						"Ном":   "@row",
						"Товар": "Номенклатура",
						"Сум":   "Сумма | number:2",
					},
				},
			},
		},
	}
}

func TestBuildSheetBasic(t *testing.T) {
	doc, err := BuildSheet(miniLayout(), buildSheetCtx())
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}

	text := doc.TextString()
	for _, want := range []string{"Накладная № 001", "ИП Иванов", "Стол", "Стул", "100.00", "250.00", "Итого:", "350.00"} {
		if !strings.Contains(text, want) {
			t.Errorf("BuildSheet output missing %q\n---\n%s", want, text)
		}
	}
}

func TestBuildSheetRowExpansion(t *testing.T) {
	doc, err := BuildSheet(miniLayout(), buildSheetCtx())
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	// Найдём строки с номерами 1 и 2 (из @row).
	text := doc.TextString()
	lines := strings.Split(text, "\n")
	var row1, row2 bool
	for _, ln := range lines {
		if strings.HasPrefix(ln, "1\t") && strings.Contains(ln, "Стол") {
			row1 = true
		}
		if strings.HasPrefix(ln, "2\t") && strings.Contains(ln, "Стул") {
			row2 = true
		}
	}
	if !row1 || !row2 {
		t.Fatalf("repeat rows not expanded by @row (row1=%v row2=%v)\n%s", row1, row2, text)
	}
}

func TestBuildSheetHeaderArea(t *testing.T) {
	doc, err := BuildSheet(miniLayout(), buildSheetCtx())
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	if !doc.RepeatHeader || doc.HeaderArea == nil {
		t.Fatalf("repeat_header not registered: RepeatHeader=%v HeaderArea=%v", doc.RepeatHeader, doc.HeaderArea)
	}
	if doc.HeaderArea.Name != "ШапкаТаблицы" {
		t.Fatalf("HeaderArea name = %q (want ШапкаТаблицы)", doc.HeaderArea.Name)
	}
}

func TestBuildSheetDefaultSequence(t *testing.T) {
	// Без Binding.Sequence — порядок Areas. Без Repeat все области выводятся один раз.
	lt := &LayoutTemplate{
		Areas: []*LayoutArea{
			{Name: "A", Rows: []LayoutRow{{Cells: []LayoutCell{{Text: "alpha"}}}}},
			{Name: "B", Rows: []LayoutRow{{Cells: []LayoutCell{{Text: "beta"}}}}},
		},
	}
	doc, err := BuildSheet(lt, &RenderContext{})
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	text := doc.TextString()
	ai := strings.Index(text, "alpha")
	bi := strings.Index(text, "beta")
	if ai == -1 || bi == -1 || ai > bi {
		t.Fatalf("default sequence order wrong: alpha@%d beta@%d\n%s", ai, bi, text)
	}
}

// TestBuildSheetEndToEnd: декларативный макет с page + per-side границами
// рендерится в HTML и PDF без ошибок, кириллица попадает в HTML.
func TestBuildSheetEndToEnd(t *testing.T) {
	lt := &LayoutTemplate{
		Name:     "УПД",
		Document: "Реализация",
		Page:     &sheet.PageSetup{Orientation: "landscape", Format: "A4", MarginsMM: sheet.Margins{Top: 5, Bottom: 5, Left: 8, Right: 8}},
		Columns:  []LayoutColumn{{Width: "40mm"}, {Width: "auto"}},
		Areas: []*LayoutArea{
			{
				Name: "Шапка",
				Rows: []LayoutRow{
					{Cells: []LayoutCell{
						{Text: "Счёт № {{Номер}}", ColSpan: 2, Bold: true,
							Borders: &CellBorders{Left: "thick", Top: "thick", Right: "thick", Bottom: "medium"}},
					}},
				},
			},
		},
		Binding: &Binding{Sequence: []string{"Шапка"}},
	}
	ctx := &RenderContext{Document: map[string]any{"Номер": "42"}}

	doc, err := BuildSheet(lt, ctx)
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	if doc.Page.Orientation != "landscape" {
		t.Errorf("page orientation not applied: %+v", doc.Page)
	}
	html := doc.HTML(sheet.HTMLOptions{})
	if !strings.Contains(html, "Счёт № 42") {
		t.Errorf("HTML missing interpolated text: %s", html)
	}
	if !strings.Contains(html, "border-left:2px solid") {
		t.Errorf("HTML missing per-side thick border")
	}
	pdf, err := doc.PDF(sheet.PDFOptions{Title: "УПД"})
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Errorf("PDF output not a PDF (prefix=%q)", pdf[:min(8, len(pdf))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestBuildSheetColumnWidths(t *testing.T) {
	lt := &LayoutTemplate{
		Columns: []LayoutColumn{
			{Width: "120px"},
			{Width: "30mm"},
			{Width: "auto"},
		},
		Areas: []*LayoutArea{
			{Name: "A", Rows: []LayoutRow{{Cells: []LayoutCell{{Text: "x"}, {Text: "y"}, {Text: "z"}}}}},
		},
	}
	doc, err := BuildSheet(lt, &RenderContext{})
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	if w := doc.ColumnWidth(1); w != 120 {
		t.Errorf("col1 width = %v (want 120px)", w)
	}
	// 30mm ≈ 113.39px
	if w := doc.ColumnWidth(2); w < 113 || w > 114 {
		t.Errorf("col2 width = %v (want ~113.4 from 30mm)", w)
	}
	if w := doc.ColumnWidth(3); w != 0 {
		t.Errorf("col3 (auto) width = %v (want 0)", w)
	}
}

// areaCellText возвращает текст ячейки области по относительным координатам
// (r,c) или "" если ячейки нет.
func areaCellText(sa *sheet.Area, r, c int) string {
	cell, ok := sa.Cells[fmt.Sprintf("%d,%d", r, c)]
	if !ok || cell == nil {
		return ""
	}
	return cell.Text
}

// TestBuildAreaCellsRowSpanShift проверяет, что ячейка следующей строки НЕ
// затирается позицией, перекрытой RowSpan-ячейкой из строки выше.
//
//	Row0: [A rowspan=2] [B]
//	Row1: [C]
//
// C должна встать в (1,1), а не в (1,0) (которую накрывает спан A), иначе при
// HTML-рендере covered-карта прячет C.
func TestBuildAreaCellsRowSpanShift(t *testing.T) {
	area := &LayoutArea{
		Name: "Шапка",
		Rows: []LayoutRow{
			{Cells: []LayoutCell{
				{Text: "A", RowSpan: 2},
				{Text: "B"},
			}},
			{Cells: []LayoutCell{
				{Text: "C"},
			}},
		},
	}
	sa := BuildAreaCells(area)

	if got := areaCellText(sa, 0, 0); got != "A" {
		t.Errorf("(0,0) = %q (want A)", got)
	}
	if got := areaCellText(sa, 0, 1); got != "B" {
		t.Errorf("(0,1) = %q (want B)", got)
	}
	// Главное: C НЕ в (1,0) (накрыта спаном A), а в (1,1).
	if got := areaCellText(sa, 1, 0); got != "" {
		t.Errorf("(1,0) = %q (must be empty — covered by rowspan A)", got)
	}
	if got := areaCellText(sa, 1, 1); got != "C" {
		t.Errorf("(1,1) = %q (want C — shifted past rowspan A)", got)
	}

	// HTML-вывод BuildSheet должен содержать текст C (covered-карта не должна его прятать).
	lt := &LayoutTemplate{Areas: []*LayoutArea{area}, Binding: &Binding{Sequence: []string{"Шапка"}}}
	doc, err := BuildSheet(lt, &RenderContext{})
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	html := doc.HTML(sheet.HTMLOptions{})
	for _, want := range []string{">A<", ">B<", ">C<"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q\n%s", want, html)
		}
	}
}

// TestBuildAreaCellsTwoLevelHeader моделирует двухуровневую шапку УПД-стиля:
//
//	Row0: [A rowspan=2] [B colspan=2]
//	Row1: [B1] [B2]
//
// Все тексты должны попасть в вывод, B1/B2 — под B (колонки 1 и 2),
// A занимает колонку 0 на обе строки.
func TestBuildAreaCellsTwoLevelHeader(t *testing.T) {
	area := &LayoutArea{
		Name: "ШапкаТаблицы",
		Rows: []LayoutRow{
			{Cells: []LayoutCell{
				{Text: "A", RowSpan: 2},
				{Text: "B", ColSpan: 2},
			}},
			{Cells: []LayoutCell{
				{Text: "B1"},
				{Text: "B2"},
			}},
		},
	}
	sa := BuildAreaCells(area)

	checks := []struct {
		r, c int
		want string
	}{
		{0, 0, "A"},
		{0, 1, "B"},
		{1, 1, "B1"}, // под B colspan — первая колонка после спана A
		{1, 2, "B2"},
	}
	for _, ch := range checks {
		if got := areaCellText(sa, ch.r, ch.c); got != ch.want {
			t.Errorf("(%d,%d) = %q (want %q)", ch.r, ch.c, got, ch.want)
		}
	}
	// (1,0) накрыта rowspan A — пусто.
	if got := areaCellText(sa, 1, 0); got != "" {
		t.Errorf("(1,0) = %q (must be empty — covered by rowspan A)", got)
	}

	lt := &LayoutTemplate{Areas: []*LayoutArea{area}, Binding: &Binding{Sequence: []string{"ШапкаТаблицы"}}}
	doc, err := BuildSheet(lt, &RenderContext{})
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	html := doc.HTML(sheet.HTMLOptions{})
	for _, want := range []string{">A<", ">B<", ">B1<", ">B2<"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q\n%s", want, html)
		}
	}
}
