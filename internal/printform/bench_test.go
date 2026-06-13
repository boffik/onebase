package printform

import (
	"fmt"
	"testing"

	"github.com/ivantit66/onebase/internal/sheet"
)

// bench_test.go — перф-прогон декларативного движка на «накладной 1000 строк»
// (план 64, этап 7.3). Синтетический документ: шапка + повтор-строка по ТЧ с
// 1000 позиций + итог. Замеряем BuildSheet (раскладка модели), HTML и PDF
// (полный рендер). Цель — убедиться, что нет O(N²) горячих точек и PDF 1000
// строк укладывается в разумное время.

// benchCtx1000 строит контекст с табличной частью на n строк.
func benchCtx1000(n int) *RenderContext {
	rows := make([]map[string]any, n)
	refs := map[string]map[string]any{
		"ref-buyer": {"наименование": "ООО «Большой Покупатель имени Длинного Наименования»"},
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("ref-g%d", i)
		refs[id] = map[string]any{"наименование": fmt.Sprintf("Номенклатурная позиция №%d со средне-длинным наименованием", i)}
		rows[i] = map[string]any{
			"Номенклатура": id,
			"Количество":   float64(i%10 + 1),
			"Цена":         float64(100 + i),
			"Сумма":        float64((i%10 + 1) * (100 + i)),
		}
	}
	return &RenderContext{
		Document: map[string]any{
			"Номер":      "ПО-100020",
			"Дата":       "2026-06-13",
			"Покупатель": "ref-buyer",
		},
		Refs:       refs,
		TableParts: map[string][]map[string]any{"Товары": rows},
	}
}

// benchLayout1000 — макет с repeat-строкой по ТЧ и итогом (Итог.Товары.Сумма).
func benchLayout1000() *LayoutTemplate {
	return &LayoutTemplate{
		Name:     "Накладная",
		Document: "Реализация",
		Areas: []*LayoutArea{
			{Name: "Шапка", Rows: []LayoutRow{
				{Cells: []LayoutCell{{Text: "Накладная № {{Номер}} от {{Дата}}", ColSpan: 4, Bold: true, Align: "center"}}},
				{Cells: []LayoutCell{{Text: "Покупатель:"}, {Parameter: "Покупатель", ColSpan: 3}}},
			}},
			{Name: "ШапкаТаблицы", Rows: []LayoutRow{
				{Cells: []LayoutCell{
					{Text: "№", Bold: true}, {Text: "Товар", Bold: true},
					{Text: "Кол-во", Bold: true}, {Text: "Сумма", Bold: true},
				}},
			}},
			{Name: "Строка", Rows: []LayoutRow{
				{Cells: []LayoutCell{
					{Parameter: "Ном"}, {Parameter: "Товар"},
					{Parameter: "Кво", Align: "right"}, {Parameter: "Сум", Align: "right"},
				}},
			}},
			{Name: "Итог", Rows: []LayoutRow{
				{Cells: []LayoutCell{{Text: "Итого:", ColSpan: 3, Align: "right", Bold: true}, {Parameter: "Всего", Align: "right", Bold: true}}},
			}},
		},
		Columns: []LayoutColumn{
			{Width: "12mm"}, {Width: "110mm"}, {Width: "25mm"}, {Width: "40mm"},
		},
		Page: &sheet.PageSetup{Orientation: "portrait", Format: "A4", MarginsMM: sheet.Margins{Top: 10, Bottom: 10, Left: 10, Right: 10}},
		Binding: &Binding{
			Sequence:     []string{"Шапка", "ШапкаТаблицы", "Строка", "Итог"},
			RepeatHeader: "ШапкаТаблицы",
			Parameters: map[string]string{
				"Покупатель": "Покупатель.наименование",
				"Всего":      "Итог.Товары.Сумма | number:2",
			},
			Repeat: []RepeatBinding{{
				Area:   "Строка",
				Source: "Товары",
				Parameters: map[string]string{
					"Ном":   "@row",
					"Товар": "Номенклатура.наименование",
					"Кво":   "Количество",
					"Сум":   "Сумма | number:2",
				},
			}},
		},
	}
}

func BenchmarkBuildSheet1000(b *testing.B) {
	lt := benchLayout1000()
	ctx := benchCtx1000(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, err := BuildSheet(lt, ctx)
		if err != nil {
			b.Fatal(err)
		}
		_ = doc
	}
}

func BenchmarkHTML1000(b *testing.B) {
	lt := benchLayout1000()
	ctx := benchCtx1000(1000)
	doc, err := BuildSheet(lt, ctx)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doc.HTMLString()
	}
}

func BenchmarkPDF1000(b *testing.B) {
	lt := benchLayout1000()
	ctx := benchCtx1000(1000)
	doc, err := BuildSheet(lt, ctx)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := doc.PDF(sheet.PDFOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}
