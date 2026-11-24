package interpreter

import (
	"testing"

	"github.com/boffik/onebase/internal/dsl/lexer"
	"github.com/boffik/onebase/internal/dsl/parser"
)

func evalChartFunc(t *testing.T, code string) any {
	t.Helper()
	l := lexer.New(code, "<test>")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	proc := prog.Procedures[0]
	i := New()
	this := &MapThis{M: map[string]any{}}
	vars := map[string]any{}
	for k, v := range NewChartFunctions() {
		vars[k] = v
	}
	var result any
	if err := i.RunWithResult(proc, this, &result, vars); err != nil {
		t.Fatalf("run: %v", err)
	}
	return result
}

func TestChart_BarChart(t *testing.T) {
	result := evalChartFunc(t, `Функция Тест()
  Д = Новый Диаграмма;
  Д.Заголовок = "Продажи";
  Д.Тип = "Гистограмма";
  С1 = Д.Серии.Добавить();
  С1.Имя = "Выручка";
  Т1 = Д.Точки.Добавить();
  Т1.Значение = "Январь";
  Т2 = Д.Точки.Добавить();
  Т2.Значение = "Февраль";
  С1.УстановитьЗначение(Т1, 100);
  С1.УстановитьЗначение(Т2, 200);
  Возврат Д;
КонецФункции`)

	chart, ok := result.(*Chart)
	if !ok {
		t.Fatalf("expected *Chart, got %T", result)
	}
	opt := chart.ToEChartsOption()

	if opt["title"].(map[string]any)["text"] != "Продажи" {
		t.Errorf("title mismatch: %v", opt["title"])
	}
	series := opt["series"].([]any)
	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}
	s := series[0].(map[string]any)
	if s["type"] != "bar" {
		t.Errorf("expected bar, got %v", s["type"])
	}
	if s["name"] != "Выручка" {
		t.Errorf("expected Выручка, got %v", s["name"])
	}
	data := s["data"].([]float64)
	if len(data) != 2 || data[0] != 100 || data[1] != 200 {
		t.Errorf("data mismatch: %v", data)
	}
	xAxis := opt["xAxis"].(map[string]any)
	cats := xAxis["data"].([]string)
	if len(cats) != 2 || cats[0] != "Январь" || cats[1] != "Февраль" {
		t.Errorf("categories mismatch: %v", cats)
	}
}

func TestChart_PieChart(t *testing.T) {
	result := evalChartFunc(t, `Функция Тест()
  Д = Новый Диаграмма;
  Д.Тип = "Круговая";
  С1 = Д.Серии.Добавить();
  С1.Имя = "Доля";
  Т1 = Д.Точки.Добавить();
  Т1.Значение = "Товар А";
  Т2 = Д.Точки.Добавить();
  Т2.Значение = "Товар Б";
  С1.УстановитьЗначение(Т1, 60);
  С1.УстановитьЗначение(Т2, 40);
  Возврат Д;
КонецФункции`)

	chart := result.(*Chart)
	opt := chart.ToEChartsOption()

	series := opt["series"].([]any)
	s := series[0].(map[string]any)
	if s["type"] != "pie" {
		t.Errorf("expected pie, got %v", s["type"])
	}
	data := s["data"].([]map[string]any)
	if len(data) != 2 {
		t.Fatalf("expected 2 data points, got %d", len(data))
	}
	if data[0]["name"] != "Товар А" || data[0]["value"] != 60.0 {
		t.Errorf("first point mismatch: %v", data[0])
	}
	if data[1]["name"] != "Товар Б" || data[1]["value"] != 40.0 {
		t.Errorf("second point mismatch: %v", data[1])
	}
	// No xAxis/yAxis for pie
	if _, has := opt["xAxis"]; has {
		t.Error("pie chart should not have xAxis")
	}
}

func TestChart_LineChart(t *testing.T) {
	result := evalChartFunc(t, `Функция Тест()
  Д = Новый Диаграмма;
  Д.Тип = "Линейная";
  С1 = Д.Серии.Добавить();
  С1.Имя = "Тренд";
  Т1 = Д.Точки.Добавить();
  Т1.Значение = "Q1";
  Т2 = Д.Точки.Добавить();
  Т2.Значение = "Q2";
  С1.УстановитьЗначение(Т1, 10);
  С1.УстановитьЗначение(Т2, 25);
  Возврат Д;
КонецФункции`)

	chart := result.(*Chart)
	opt := chart.ToEChartsOption()

	series := opt["series"].([]any)
	s := series[0].(map[string]any)
	if s["type"] != "line" {
		t.Errorf("expected line, got %v", s["type"])
	}
}

func TestChart_MultipleSeries(t *testing.T) {
	result := evalChartFunc(t, `Функция Тест()
  Д = Новый Диаграмма;
  С1 = Д.Серии.Добавить();
  С1.Имя = "Выручка";
  С2 = Д.Серии.Добавить();
  С2.Имя = "Прибыль";
  Т1 = Д.Точки.Добавить();
  Т1.Значение = "А";
  Т2 = Д.Точки.Добавить();
  Т2.Значение = "Б";
  С1.УстановитьЗначение(Т1, 100);
  С1.УстановитьЗначение(Т2, 200);
  С2.УстановитьЗначение(Т1, 30);
  С2.УстановитьЗначение(Т2, 60);
  Возврат Д;
КонецФункции`)

	chart := result.(*Chart)
	opt := chart.ToEChartsOption()

	series := opt["series"].([]any)
	if len(series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(series))
	}
	d1 := series[0].(map[string]any)["data"].([]float64)
	d2 := series[1].(map[string]any)["data"].([]float64)
	if d1[0] != 100 || d1[1] != 200 {
		t.Errorf("series1 data: %v", d1)
	}
	if d2[0] != 30 || d2[1] != 60 {
		t.Errorf("series2 data: %v", d2)
	}

	legend := opt["legend"].(map[string]any)
	ldata := legend["data"].([]string)
	if len(ldata) != 2 || ldata[0] != "Выручка" || ldata[1] != "Прибыль" {
		t.Errorf("legend data: %v", ldata)
	}
}

func TestChart_DefaultType(t *testing.T) {
	c := NewChart()
	opt := c.ToEChartsOption()
	series := opt["series"]
	if series != nil {
		// Empty chart with default type still produces valid structure
		t.Logf("empty chart option: %v", opt)
	}
}

func TestChart_Properties(t *testing.T) {
	c := NewChart()
	c.Set("Ширина", "800px")
	c.Set("Высота", "600px")
	c.Set("Легенда", false)
	c.Set("Подписи", true)

	if c.Get("ширина") != "800px" {
		t.Errorf("width: %v", c.Get("ширина"))
	}
	if c.Get("высота") != "600px" {
		t.Errorf("height: %v", c.Get("высота"))
	}
	if c.Get("легенда") != false {
		t.Errorf("legend: %v", c.Get("легенда"))
	}
	if c.Get("подписи") != true {
		t.Errorf("labels: %v", c.Get("подписи"))
	}
}
