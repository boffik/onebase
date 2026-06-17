package ui

import (
	"fmt"
	"html"
	"html/template"
	"net/url"
	"strings"

	"github.com/ivantit66/onebase/internal/report"
	"github.com/ivantit66/onebase/internal/report/compose"
)

// renderComposedTable строит единую <table> с раскрываемыми группами,
// подитогами, общим итогом и условным оформлением деталей.
func renderComposedTable(res *compose.Result, spec *report.Composition) template.HTML {
	var b strings.Builder
	b.WriteString(`<table class="report-composed">`)
	b.WriteString(`<thead><tr><th>` + html.EscapeString(strings.Join(spec.Groupings, " / ")) + `</th>`)
	for _, m := range spec.Measures {
		b.WriteString(`<th class="num" style="` + html.EscapeString(measureAlign(m)) + `">` + html.EscapeString(measureTitle(m)) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, g := range res.Groups {
		writeGroup(&b, g, spec, 0, "")
	}
	if spec.Totals.Grand {
		b.WriteString(`<tr class="grand"><td>ВСЕГО</td>`)
		for _, m := range spec.Measures {
			b.WriteString(`<td class="num" style="` + html.EscapeString(measureAlign(m)) + `">` + html.EscapeString(fmtMeasure(res.Grand[m.Field], m)) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return template.HTML(b.String())
}

func writeGroup(b *strings.Builder, g *compose.Group, spec *report.Composition, level int, path string) {
	gp := path + "/" + pathSeg(fmtVal(g.Key))
	pad := fmt.Sprintf("padding-left:%dpx", 8+level*18)
	rowStyle := cssOf(g.Styles[""])
	fmt.Fprintf(b, `<tr class="grp" data-group="%s" data-level="%d" style="%s"><td style="%s">▼ %s</td>`,
		html.EscapeString(gp), level, html.EscapeString(rowStyle), pad, html.EscapeString(fmtVal(g.Key)))
	for _, m := range spec.Measures {
		cell := joinStyles(measureAlign(m), cssOf(g.Styles[m.Field]))
		b.WriteString(`<td class="num" style="` + html.EscapeString(cell) + `">` + html.EscapeString(fmtMeasure(g.Subtotals[m.Field], m)) + `</td>`)
	}
	b.WriteString(`</tr>`)
	for _, ch := range g.Children {
		writeGroup(b, ch, spec, level+1, gp)
	}
	for _, d := range g.Details {
		writeDetail(b, d, spec, level+1, gp)
	}
	if spec.Totals.Subtotals {
		fmt.Fprintf(b, `<tr class="subtotal" data-parent="%s" style="%s"><td style="padding-left:%dpx">··· Итого: %s ···</td>`,
			html.EscapeString(gp), html.EscapeString(rowStyle), 8+(level+1)*18, html.EscapeString(fmtVal(g.Key)))
		for _, m := range spec.Measures {
			cell := joinStyles(measureAlign(m), cssOf(g.Styles[m.Field]))
			b.WriteString(`<td class="num" style="` + html.EscapeString(cell) + `">` + html.EscapeString(fmtMeasure(g.Subtotals[m.Field], m)) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
}

func writeDetail(b *strings.Builder, d compose.DetailRow, spec *report.Composition, level int, path string) {
	rowStyle := cssOf(d.Styles[""])
	fmt.Fprintf(b, `<tr class="det" data-parent="%s" style="%s">`, html.EscapeString(path), html.EscapeString(rowStyle))
	// Первая ячейка: ссылка-расшифровка на исходный документ (если настроено)
	if spec.DetailLink != "" {
		if v := fmtVal(d.Values[spec.DetailLink]); v != "" {
			href := "/ui/document/" + url.PathEscape(spec.DetailEntity) + "/" + url.PathEscape(v)
			fmt.Fprintf(b, `<td style="padding-left:%dpx"><a href="%s">→</a></td>`, 8+level*18, html.EscapeString(href))
		} else {
			fmt.Fprintf(b, `<td style="padding-left:%dpx"></td>`, 8+level*18)
		}
	} else {
		fmt.Fprintf(b, `<td style="padding-left:%dpx"></td>`, 8+level*18)
	}
	for _, m := range spec.Measures {
		cell := joinStyles(measureAlign(m), cssOf(d.Styles[m.Field]))
		b.WriteString(`<td class="num" style="` + html.EscapeString(cell) + `">` + html.EscapeString(fmtMeasure(d.Values[m.Field], m)) + `</td>`)
	}
	b.WriteString(`</tr>`)
}

func cssOf(s report.CellStyle) string {
	var p []string
	if s.Color != "" {
		p = append(p, "color:"+s.Color)
	}
	if s.Background != "" {
		p = append(p, "background:"+s.Background)
	}
	if s.Bold {
		p = append(p, "font-weight:bold")
	}
	if s.Italic {
		p = append(p, "font-style:italic")
	}
	return strings.Join(p, ";")
}

func measureTitle(m report.Measure) string {
	if m.Title != "" {
		return m.Title
	}
	return m.Field
}

// measureAlign возвращает CSS-свойство выравнивания для ячейки показателя.
// По умолчанию (пустое Align) — вправо, как было исторически.
func measureAlign(m report.Measure) string {
	switch m.Align {
	case "left", "center":
		return "text-align:" + m.Align
	default:
		return "text-align:right"
	}
}

// joinStyles объединяет два CSS-стиля через ";", пропуская пустые части.
func joinStyles(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + ";" + b
}

// buildComposedChart строит ECharts-option из верхнего уровня группировки.
// Формат совпадает с тем, что отдаёт ChartProc (слот ChartOption шаблона).
func buildComposedChart(res *compose.Result, c *report.ChartSpec) map[string]any {
	if c == nil || len(res.Groups) == 0 {
		return nil
	}
	if c.Type == "pie" {
		// Круговая: один ряд из пар {name,value} по первому показателю.
		// Несколько series для pie в v1 не поддерживаются (берём firstSeries).
		pie := make([]any, 0, len(res.Groups))
		for _, g := range res.Groups {
			pie = append(pie, map[string]any{"name": fmtVal(g.Key), "value": numFor(g.Subtotals[firstSeries(c)])})
		}
		return map[string]any{
			"tooltip": map[string]any{"trigger": "item"},
			"series":  []any{map[string]any{"type": "pie", "data": pie}},
		}
	}
	cats := make([]string, 0, len(res.Groups))
	for _, g := range res.Groups {
		cats = append(cats, fmtVal(g.Key))
	}
	var series []any
	for _, sf := range c.Series {
		data := make([]any, 0, len(res.Groups))
		for _, g := range res.Groups {
			data = append(data, numFor(g.Subtotals[sf]))
		}
		series = append(series, map[string]any{"name": sf, "type": chartType(c.Type), "data": data})
	}
	return map[string]any{
		"tooltip": map[string]any{"trigger": "axis"},
		"series":  series,
		"xAxis":   map[string]any{"type": "category", "data": cats},
		"yAxis":   map[string]any{"type": "value"},
	}
}

func chartType(t string) string {
	if t == "line" {
		return "line"
	}
	return "bar"
}

func firstSeries(c *report.ChartSpec) string {
	if len(c.Series) > 0 {
		return c.Series[0]
	}
	return ""
}

func numFor(v any) any {
	if d, ok := compose.ExportToDecimal(v); ok {
		f, _ := d.Float64()
		return f
	}
	return float64(0)
}

func fmtVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// fmtMeasure форматирует значение показателя с учётом поля Format.
// Если Format пустой или значение не числовое — возвращает fmtVal(v).
func fmtMeasure(v any, m report.Measure) string {
	if m.Format != "" {
		if d, ok := compose.ExportToDecimal(v); ok {
			return compose.FormatNumber(d, m.Format)
		}
	}
	return fmtVal(v)
}

// pathSeg экранирует сегмент пути группы для data-group/data-parent. Без этого
// «/» внутри значения группировки ломает префиксное сопоставление при
// сворачивании: сиблинг «A/Б» (data-group "/A/Б") ложно прятался при
// сворачивании «A» (селектор [data-group^="/A/"]). Видимая подпись — сырая.
func pathSeg(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	return strings.ReplaceAll(s, "/", "%2F")
}
