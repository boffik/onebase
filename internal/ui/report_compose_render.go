package ui

import (
	"fmt"
	"html"
	"html/template"
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
		b.WriteString(`<th class="num">` + html.EscapeString(measureTitle(m)) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, g := range res.Groups {
		writeGroup(&b, g, spec, 0, "")
	}
	if spec.Totals.Grand {
		b.WriteString(`<tr class="grand"><td>ВСЕГО</td>`)
		for _, m := range spec.Measures {
			b.WriteString(`<td class="num">` + html.EscapeString(fmtVal(res.Grand[m.Field])) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return template.HTML(b.String())
}

func writeGroup(b *strings.Builder, g *compose.Group, spec *report.Composition, level int, path string) {
	gp := path + "/" + fmtVal(g.Key)
	pad := fmt.Sprintf("padding-left:%dpx", 8+level*18)
	fmt.Fprintf(b, `<tr class="grp" data-group="%s" data-level="%d"><td style="%s">▼ %s</td>`,
		html.EscapeString(gp), level, pad, html.EscapeString(fmtVal(g.Key)))
	for _, m := range spec.Measures {
		b.WriteString(`<td class="num">` + html.EscapeString(fmtVal(g.Subtotals[m.Field])) + `</td>`)
	}
	b.WriteString(`</tr>`)
	for _, ch := range g.Children {
		writeGroup(b, ch, spec, level+1, gp)
	}
	for _, d := range g.Details {
		writeDetail(b, d, spec, level+1, gp)
	}
	if spec.Totals.Subtotals {
		fmt.Fprintf(b, `<tr class="subtotal" data-parent="%s"><td style="padding-left:%dpx">··· Итого: %s ···</td>`,
			html.EscapeString(gp), 8+(level+1)*18, html.EscapeString(fmtVal(g.Key)))
		for _, m := range spec.Measures {
			b.WriteString(`<td class="num">` + html.EscapeString(fmtVal(g.Subtotals[m.Field])) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
}

func writeDetail(b *strings.Builder, d compose.DetailRow, spec *report.Composition, level int, path string) {
	rowStyle := cssOf(d.Styles[""])
	fmt.Fprintf(b, `<tr class="det" data-parent="%s" style="%s">`, html.EscapeString(path), html.EscapeString(rowStyle))
	fmt.Fprintf(b, `<td style="padding-left:%dpx"></td>`, 8+level*18)
	for _, m := range spec.Measures {
		cell := cssOf(d.Styles[m.Field])
		b.WriteString(`<td class="num" style="` + html.EscapeString(cell) + `">` + html.EscapeString(fmtVal(d.Values[m.Field])) + `</td>`)
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

func fmtVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
