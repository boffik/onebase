package launcher

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/report"
)

func renderConfiguratorReport(t *testing.T, comp *report.Composition) string {
	t.Helper()
	data := &configuratorData{
		Base: &Base{ID: "b", Name: "Т", ConfigSource: "file"}, Lang: "ru", Tab: "tree",
		Reports: []cfgReport{{Name: "Прод", Composition: comp}},
	}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "tab-tree", data); err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	return buf.String()
}

func TestReportBuilderRender(t *testing.T) {
	html := renderConfiguratorReport(t, &report.Composition{
		Groupings: []string{"Менеджер"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum"}},
	})
	for _, want := range []string{"comp.present", "comp.grouping.0", "Структура", "obj-tab"} {
		if !strings.Contains(html, want) {
			t.Fatalf("нет %q", want)
		}
	}
	// nil composition не должен паниковать
	_ = renderConfiguratorReport(t, nil)
}

func TestReportBuilderCondTab(t *testing.T) {
	html := renderConfiguratorReport(t, &report.Composition{
		Groupings:   []string{"М"},
		Conditional: []report.CondRule{{When: "Сумма < 0", Style: report.CellStyle{Color: "#c00"}}},
	})
	for _, want := range []string{"Оформление", "comp.cond.0.when", "ot-rep-cond-"} {
		if !strings.Contains(html, want) {
			t.Fatalf("нет %q", want)
		}
	}
}
