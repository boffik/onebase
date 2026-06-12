package printform

import (
	"strings"
	"testing"
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
