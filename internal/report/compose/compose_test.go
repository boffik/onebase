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
	w, werr := decimal.NewFromString(want)
	if !ok || werr != nil || !d.Equal(w) {
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

func TestNestedAndDetails(t *testing.T) {
	rows := []Row{
		{"М": "И", "К": "Р", "Сумма": "600"},
		{"М": "И", "К": "Р", "Сумма": "380"},
		{"М": "И", "К": "П", "Сумма": "270"},
	}
	spec := report.Composition{
		Groupings: []string{"М", "К"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum"}},
		Detail:    true,
	}
	res, err := Compose(rows, spec, noEval{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Groups) != 1 || len(res.Groups[0].Children) != 2 {
		t.Fatalf("tree: %+v", res.Groups)
	}
	rom := res.Groups[0].Children[0]
	decEq(t, rom.Subtotals["Сумма"], "980")
	if len(rom.Details) != 2 {
		t.Fatalf("details=%d", len(rom.Details))
	}
	if rom.Details[0].Values["Сумма"] != "600" {
		t.Fatalf("detail val: %v", rom.Details[0].Values["Сумма"])
	}
}

func TestAggregates(t *testing.T) {
	rows := []Row{
		{"Г": "A", "X": "10.50"},
		{"Г": "A", "X": "4.50"},
	}
	mk := func(agg string) any {
		spec := report.Composition{Groupings: []string{"Г"}, Measures: []report.Measure{{Field: "X", Agg: agg}}}
		res, _ := Compose(rows, spec, noEval{})
		return res.Groups[0].Subtotals["X"]
	}
	decEq(t, mk("sum"), "15")
	decEq(t, mk("avg"), "7.5")
	decEq(t, mk("min"), "4.5")
	decEq(t, mk("max"), "10.5")
	if c, _ := mk("count").(int64); c != 2 {
		t.Fatalf("count=%v", mk("count"))
	}
}

func TestSort(t *testing.T) {
	rows := []Row{
		{"М": "А", "Сумма": "100"},
		{"М": "Б", "Сумма": "300"},
		{"М": "В", "Сумма": "200"},
	}
	spec := report.Composition{
		Groupings: []string{"М"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum"}},
		Sort:      []report.SortKey{{Field: "Сумма", Dir: "desc"}},
	}
	res, _ := Compose(rows, spec, noEval{})
	got := []any{res.Groups[0].Key, res.Groups[1].Key, res.Groups[2].Key}
	if got[0] != "Б" || got[1] != "В" || got[2] != "А" {
		t.Fatalf("order by subtotal desc: %v", got)
	}
}
