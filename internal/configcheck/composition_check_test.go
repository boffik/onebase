package configcheck

import (
	"testing"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/report"
)

func projWith(c *report.Composition) *project.Project {
	return &project.Project{Reports: []*report.Report{{Name: "R", Query: "ВЫБРАТЬ 1", Composition: c}}}
}

func TestCompositionOK(t *testing.T) {
	c := &report.Composition{
		Groupings:   []string{"М"},
		Measures:    []report.Measure{{Field: "Сумма", Agg: "sum"}},
		Sort:        []report.SortKey{{Field: "Сумма", Dir: "desc"}},
		Chart:       &report.ChartSpec{Type: "bar", Category: "М", Series: []string{"Сумма"}},
		Conditional: []report.CondRule{{When: "Сумма < 0"}},
	}
	if iss := CheckReportComposition(projWith(c)); len(iss) != 0 {
		t.Fatalf("ожидали 0 проблем: %+v", iss)
	}
}

func TestCompositionBad(t *testing.T) {
	c := &report.Composition{
		Groupings:   []string{"М"},
		Measures:    []report.Measure{{Field: "Сумма", Agg: "wat"}},
		Chart:       &report.ChartSpec{Type: "donut", Category: "Нет", Series: []string{"Икс"}},
		Conditional: []report.CondRule{{When: "Сумма < "}}, // битое выражение
	}
	iss := CheckReportComposition(projWith(c))
	if len(iss) < 3 {
		t.Fatalf("ожидали несколько проблем, получили %d: %+v", len(iss), iss)
	}
}
