package compose

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/ivantit66/onebase/internal/report"
)

// noEval — заглушка Evaluator: оформление не применяется (для тестов без условий).
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

func TestGroupByDecimalKey(t *testing.T) {
	// Две равные по значению decimal должны попасть в одну группу: ключ
	// нормализуется в строку, а не сравнивается по указателю big.Int.
	rows := []Row{
		{"Год": decimal.NewFromInt(2026), "Сумма": "10"},
		{"Год": decimal.NewFromInt(2026), "Сумма": "20"},
		{"Год": decimal.NewFromInt(2025), "Сумма": "5"},
	}
	spec := report.Composition{
		Groupings: []string{"Год"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum"}},
	}
	res, _ := Compose(rows, spec, noEval{})
	if len(res.Groups) != 2 {
		t.Fatalf("ожидали 2 группы по равным decimal, получили %d", len(res.Groups))
	}
	decEq(t, res.Groups[0].Subtotals["Сумма"], "30")
}
