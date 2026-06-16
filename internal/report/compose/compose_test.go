package compose

import (
	"testing"

	"github.com/ivantit66/onebase/internal/report"
)

// fakeEval — всегда совпадение по подстроке "<0" при отрицательной Сумме (для поздних тестов).
type noEval struct{}

func (noEval) EvalBool(string, Row) (bool, error) { return false, nil }

func decEq(t *testing.T, got any, want string) {
	t.Helper()
	d, ok := toDecimal(got)
	if !ok || d.String() != want {
		t.Fatalf("got %v (%T), want %s", got, got, want)
	}
}

func TestSingleGrouping(t *testing.T) {
	rows := []Row{
		{"Менеджер": "Иванов", "Сумма": "100"},
		{"Менеджер": "Иванов", "Сумма": "50"},
		{"Менеджер": "Петров", "Сумма": "30"},
	}
	spec := report.Composition{
		Groupings: []string{"Менеджер"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum"}},
		Totals:    report.Totals{Grand: true, Subtotals: true},
	}
	res, err := Compose(rows, spec, noEval{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Groups) != 2 {
		t.Fatalf("groups=%d", len(res.Groups))
	}
	if res.Groups[0].Key != "Иванов" {
		t.Fatalf("order: %v", res.Groups[0].Key)
	}
	decEq(t, res.Groups[0].Subtotals["Сумма"], "150")
	decEq(t, res.Grand["Сумма"], "180")
	if res.RowCount != 3 || res.Capped {
		t.Fatalf("rowcount=%d capped=%v", res.RowCount, res.Capped)
	}
}
