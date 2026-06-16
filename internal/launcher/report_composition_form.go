package launcher

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/ivantit66/onebase/internal/report"
	"gopkg.in/yaml.v3"
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
		color := strings.TrimSpace(f.Get(p + ".color"))
		if color == "#000000" {
			color = ""
		}
		background := strings.TrimSpace(f.Get(p + ".background"))
		if background == "#ffffff" {
			background = ""
		}
		c.Conditional = append(c.Conditional, report.CondRule{
			When:  when,
			Field: strings.TrimSpace(f.Get(p + ".field")),
			Style: report.CellStyle{
				Color:      color,
				Background: background,
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

	// Включён, но пуст → сигнал очистки (nil, true). Очищаем только когда пусто
	// ВСЁ — иначе правила оформления / сортировка / график без группировок молча
	// терялись бы при сохранении (work-in-progress конструктора).
	if len(c.Groupings) == 0 && len(c.Measures) == 0 && len(c.Conditional) == 0 && len(c.Sort) == 0 && c.Chart == nil {
		return nil, true
	}
	return c, true
}

// applyReportComposition обновляет блок composition в сыром YAML отчёта по форме.
// Существующий composition сохраняется, если в форме нет comp.present;
// перезаписывается/очищается, если present. Остальные поля отчёта не трогаются.
func applyReportComposition(raw []byte, f url.Values) ([]byte, error) {
	var doc struct {
		Name        string              `yaml:"name"`
		Title       string              `yaml:"title,omitempty"`
		Params      []map[string]any    `yaml:"params,omitempty"`
		Query       string              `yaml:"query"`
		ChartProc   string              `yaml:"chart_proc,omitempty"`
		Composition *report.Composition `yaml:"composition,omitempty"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if c, present := parseCompositionForm(f); present {
		doc.Composition = c // c==nil очищает блок
	}
	return yaml.Marshal(&doc)
}
