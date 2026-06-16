package report

import "testing"

func TestParseComposition(t *testing.T) {
	src := []byte(`
name: Прод
query: "ВЫБРАТЬ 1"
composition:
  groupings: [Менеджер, Клиент]
  measures:
    - { field: Сумма, agg: sum, title: "Сумма, ₽" }
  totals: { grand: true, subtotals: true }
  detail: true
  sort: [ { field: Сумма, dir: desc } ]
  conditional:
    - { when: "Сумма < 0", field: "", style: { color: "#c00", bold: true } }
  chart: { type: bar, category: Менеджер, series: [Сумма] }
`)
	r, err := ParseBytes(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Composition == nil {
		t.Fatal("Composition is nil")
	}
	c := r.Composition
	if len(c.Groupings) != 2 || c.Groupings[0] != "Менеджер" {
		t.Fatalf("groupings: %v", c.Groupings)
	}
	if len(c.Measures) != 1 || c.Measures[0].Field != "Сумма" || c.Measures[0].Agg != "sum" || c.Measures[0].Title != "Сумма, ₽" {
		t.Fatalf("measures: %+v", c.Measures)
	}
	if !c.Totals.Grand || !c.Totals.Subtotals || !c.Detail {
		t.Fatalf("totals/detail: %+v %v", c.Totals, c.Detail)
	}
	if len(c.Conditional) != 1 || c.Conditional[0].Field != "" || c.Conditional[0].Style.Color != "#c00" || !c.Conditional[0].Style.Bold || c.Conditional[0].Style.Italic {
		t.Fatalf("conditional: %+v", c.Conditional)
	}
	if len(c.Sort) != 1 || c.Sort[0].Field != "Сумма" || c.Sort[0].Dir != "desc" {
		t.Fatalf("sort: %+v", c.Sort)
	}
	if c.Chart == nil || c.Chart.Type != "bar" || c.Chart.Category != "Менеджер" || len(c.Chart.Series) != 1 || c.Chart.Series[0] != "Сумма" {
		t.Fatalf("chart: %+v", c.Chart)
	}
}

func TestParseNoComposition(t *testing.T) {
	r, err := ParseBytes([]byte("name: X\nquery: \"ВЫБРАТЬ 1\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Composition != nil {
		t.Fatal("Composition must be nil when absent")
	}
}
