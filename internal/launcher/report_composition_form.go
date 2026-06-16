package launcher

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/ivantit66/onebase/internal/report"
)

// parseCompositionForm собирает report.Composition из полей формы comp.*.
// (nil, false) — маркера comp.present нет (composition не трогаем).
// (nil, true)  — включён, но пуст (composition очищается).
func parseCompositionForm(f url.Values) (*report.Composition, bool) {
	if f.Get("comp.present") == "" {
		return nil, false
	}
	c := &report.Composition{}

	// Группировки
	for i := 0; ; i++ {
		v := strings.TrimSpace(f.Get("comp.grouping." + strconv.Itoa(i)))
		if v == "" {
			break
		}
		c.Groupings = append(c.Groupings, v)
	}

	// Показатели
	for i := 0; ; i++ {
		p := "comp.measure." + strconv.Itoa(i)
		fld := strings.TrimSpace(f.Get(p + ".field"))
		if fld == "" {
			break
		}
		c.Measures = append(c.Measures, report.Measure{
			Field: fld,
			Agg:   f.Get(p + ".agg"),
			Title: strings.TrimSpace(f.Get(p + ".title")),
		})
	}

	// Итоги и детальные записи
	c.Totals.Grand = f.Get("comp.totals.grand") != ""
	c.Totals.Subtotals = f.Get("comp.totals.subtotals") != ""
	c.Detail = f.Get("comp.detail") != ""

	// Сортировка
	for i := 0; ; i++ {
		p := "comp.sort." + strconv.Itoa(i)
		fld := strings.TrimSpace(f.Get(p + ".field"))
		if fld == "" {
			break
		}
		c.Sort = append(c.Sort, report.SortKey{Field: fld, Dir: f.Get(p + ".dir")})
	}

	// Условное оформление
	for i := 0; ; i++ {
		p := "comp.cond." + strconv.Itoa(i)
		when := strings.TrimSpace(f.Get(p + ".when"))
		if when == "" {
			break
		}
		c.Conditional = append(c.Conditional, report.CondRule{
			When:  when,
			Field: strings.TrimSpace(f.Get(p + ".field")),
			Style: report.CellStyle{
				Color:      strings.TrimSpace(f.Get(p + ".color")),
				Background: strings.TrimSpace(f.Get(p + ".background")),
				Bold:       f.Get(p+".bold") != "",
				Italic:     f.Get(p+".italic") != "",
			},
		})
	}

	// Диаграмма (пустой type → нет диаграммы)
	if ct := strings.TrimSpace(f.Get("comp.chart.type")); ct != "" {
		var series []string
		for _, s := range strings.Split(f.Get("comp.chart.series"), ",") {
			if s = strings.TrimSpace(s); s != "" {
				series = append(series, s)
			}
		}
		c.Chart = &report.ChartSpec{
			Type:     ct,
			Category: strings.TrimSpace(f.Get("comp.chart.category")),
			Series:   series,
		}
	}

	// Включён, но пуст → сигнал очистки (nil, true)
	if len(c.Groupings) == 0 && len(c.Measures) == 0 {
		return nil, true
	}
	return c, true
}
