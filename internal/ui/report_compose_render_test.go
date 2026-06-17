package ui

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/report"
	"github.com/ivantit66/onebase/internal/report/compose"
)

func TestRenderGroupConditional(t *testing.T) {
	// Условное оформление должно отрисовываться на строке группы и подытоге
	// (при detail:false), а не только на детальных строках.
	rows := []compose.Row{
		{"М": "Убыток", "Сумма": "-100"},
		{"М": "Прибыль", "Сумма": "50"},
	}
	spec := report.Composition{
		Groupings:   []string{"М"},
		Measures:    []report.Measure{{Field: "Сумма", Agg: "sum"}},
		Totals:      report.Totals{Subtotals: true},
		Conditional: []report.CondRule{{When: "Сумма < 0", Field: "", Style: report.CellStyle{Color: "#c00", Bold: true}}},
	}
	res, _ := compose.Compose(rows, spec, newInterpEvaluator(interpreter.New()))
	out := string(renderComposedTable(res, &spec))
	if !strings.Contains(out, "color:#c00") || !strings.Contains(out, "font-weight:bold") {
		t.Fatalf("строка убыточной группы должна иметь стиль:\n%s", out)
	}
}

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

func TestComposedPathEscaping(t *testing.T) {
	// Значение группировки с «/» не должно ломать префиксную схему путей:
	// сиблинг «Иванов/Доп» обязан иметь экранированный data-group, иначе
	// сворачивание «Иванов» ложно спрятало бы его.
	rows := []compose.Row{
		{"М": "Иванов", "Сумма": "10"},
		{"М": "Иванов/Доп", "Сумма": "20"},
	}
	spec := report.Composition{
		Groupings: []string{"М"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum"}},
	}
	res, _ := compose.Compose(rows, spec, nil)
	out := string(renderComposedTable(res, &spec))
	if !strings.Contains(out, `data-group="/Иванов"`) {
		t.Fatalf("нет группы Иванов:\n%s", out)
	}
	if !strings.Contains(out, `data-group="/Иванов%2FДоп"`) {
		t.Fatalf("сегмент с / не экранирован:\n%s", out)
	}
	if strings.Contains(out, `data-group="/Иванов/Доп"`) {
		t.Fatalf("неэкранированный путь ломает префикс-схему:\n%s", out)
	}
	if !strings.Contains(out, "Иванов/Доп") {
		t.Fatalf("видимая подпись должна быть сырой:\n%s", out)
	}
}
