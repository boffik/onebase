package ui

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/report"
	"github.com/ivantit66/onebase/internal/report/compose"
)

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
