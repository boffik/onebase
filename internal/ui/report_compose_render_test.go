package ui

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/report"
	"github.com/ivantit66/onebase/internal/report/compose"
)

func TestBuildComposedChart(t *testing.T) {
	rows := []compose.Row{{"М": "Иванов", "Сумма": "150"}, {"М": "Петров", "Сумма": "30"}}
	spec := report.Composition{
		Groupings: []string{"М"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum"}},
		Chart:     &report.ChartSpec{Type: "bar", Category: "М", Series: []string{"Сумма"}},
	}
	res, _ := compose.Compose(rows, spec, nil)
	opt := buildComposedChart(res, spec.Chart)
	if opt == nil {
		t.Fatal("nil chart option")
	}
	xAxis, _ := opt["xAxis"].(map[string]any)
	cats, _ := xAxis["data"].([]string)
	if len(cats) != 2 || cats[0] != "Иванов" {
		t.Fatalf("categories: %v", cats)
	}
}

func TestRenderComposedTable(t *testing.T) {
	rows := []compose.Row{
		{"М": "Иванов", "Сумма": "150"},
		{"М": "Петров", "Сумма": "30"},
	}
	spec := report.Composition{
		Groupings: []string{"М"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum", Title: "Сумма, ₽"}},
		Totals:    report.Totals{Grand: true, Subtotals: true},
	}
	res, _ := compose.Compose(rows, spec, nil)
	out := string(renderComposedTable(res, &spec))
	for _, want := range []string{"Иванов", "Петров", "150", "Сумма, ₽", "ВСЕГО", "data-group", "<table"} {
		if !strings.Contains(out, want) {
			t.Fatalf("HTML не содержит %q:\n%s", want, out)
		}
	}
}
