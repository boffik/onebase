package launcher

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/widget"
)

// Для графика предпросмотр строит таблицу «ось X + по колонке на серию» и
// человекочитаемую раскладку — это и делает видимым, какая колонка стала осью,
// а какие серией.
func TestBuildWidgetPreview_Chart(t *testing.T) {
	wdg := &metadata.Widget{Type: metadata.WidgetTypeChart, XField: "Месяц"}
	res := widget.Result{
		Type:  "chart",
		Title: "Приход и расход",
		Chart: &widget.ChartData{
			Kind:  "bar",
			XAxis: []string{"2026-01", "2026-02"},
			Series: []widget.ChartSeries{
				{Name: "Приход", Data: []float64{522000, 0}},
				{Name: "Расход", Data: []float64{0, 130000}},
			},
		},
	}

	pv := buildWidgetPreview(wdg, res)
	if !pv.OK || pv.Error != "" {
		t.Fatalf("ожидался успех, got OK=%v err=%q", pv.OK, pv.Error)
	}
	if got := strings.Join(pv.Columns, "|"); got != "Месяц|Приход|Расход" {
		t.Errorf("колонки = %q, want Месяц|Приход|Расход", got)
	}
	if len(pv.Rows) != 2 || strings.Join(pv.Rows[0], "|") != "2026-01|522000|0" {
		t.Errorf("первая строка неверна: %v", pv.Rows)
	}
	if pv.Chart == nil || len(pv.Chart.Series) != 2 {
		t.Fatalf("chart не проброшен: %+v", pv.Chart)
	}
	if !strings.Contains(pv.Mapping, "Ось X: Месяц") || !strings.Contains(pv.Mapping, "Приход, Расход") {
		t.Errorf("раскладка не описывает ось/серии: %q", pv.Mapping)
	}
	// Предпросмотр должен отдавать ту же опцию ECharts, что и рабочий стол,
	// чтобы рисоваться идентично пользовательскому режиму.
	if len(pv.EChartsOption) == 0 {
		t.Fatal("echarts_option не заполнен")
	}
	var opt struct {
		Series []struct {
			Type   string `json:"type"`
			Smooth bool   `json:"smooth"`
		} `json:"series"`
	}
	if err := json.Unmarshal(pv.EChartsOption, &opt); err != nil {
		t.Fatalf("echarts_option не парсится: %v", err)
	}
	if len(opt.Series) != 2 || opt.Series[0].Type != "bar" {
		t.Errorf("series ECharts неверны: %+v", opt.Series)
	}
}

// line-виджет должен давать smooth-линию — это то, чем рабочий стол рисует
// сглаженный график, и предпросмотр обязан совпадать.
func TestBuildWidgetPreview_ChartLineSmooth(t *testing.T) {
	wdg := &metadata.Widget{Type: metadata.WidgetTypeChart, XField: "Месяц"}
	res := widget.Result{
		Type: "chart",
		Chart: &widget.ChartData{
			Kind:   "line",
			XAxis:  []string{"2026-01", "2026-02"},
			Series: []widget.ChartSeries{{Name: "Приход", Data: []float64{1, 2}}},
		},
	}
	pv := buildWidgetPreview(wdg, res)
	var opt struct {
		Series []struct {
			Type   string `json:"type"`
			Smooth bool   `json:"smooth"`
		} `json:"series"`
	}
	if err := json.Unmarshal(pv.EChartsOption, &opt); err != nil {
		t.Fatalf("echarts_option не парсится: %v", err)
	}
	if len(opt.Series) != 1 || opt.Series[0].Type != "line" || !opt.Series[0].Smooth {
		t.Errorf("line должен быть smooth: %+v", opt.Series)
	}
}

func TestBuildWidgetPreview_KPI(t *testing.T) {
	wdg := &metadata.Widget{Type: metadata.WidgetTypeKPI}
	res := widget.Result{Type: "kpi", KPI: &widget.KPIResult{Display: "1 234 ₽"}}
	pv := buildWidgetPreview(wdg, res)
	if pv.KPI != "1 234 ₽" || len(pv.Rows) != 1 || pv.Rows[0][0] != "1 234 ₽" {
		t.Errorf("kpi предпросмотр неверен: %+v", pv)
	}
}

// Ошибка исполнения должна доходить до клиента, а не молчать.
func TestBuildWidgetPreview_Error(t *testing.T) {
	wdg := &metadata.Widget{Type: metadata.WidgetTypeChart}
	res := widget.Result{Type: "chart", Error: "no such table: Х"}
	pv := buildWidgetPreview(wdg, res)
	if pv.OK || pv.Error != "no such table: Х" {
		t.Errorf("ошибка не проброшена: %+v", pv)
	}
}

// Ошибку «no such column» дополняем подсказкой про миграцию (частый случай —
// БД отстала от YAML регистра); прочие ошибки и пустую строку не трогаем.
func TestHintSchemaError(t *testing.T) {
	in := "run query: SQL logic error: no such column: документ (1)"
	got := hintSchemaError(in)
	if !strings.Contains(got, in) || !strings.Contains(got, "onebase migrate") {
		t.Errorf("ожидалась подсказка про миграцию, got %q", got)
	}
	if hintSchemaError("") != "" {
		t.Error("пустую строку не трогаем")
	}
	if other := "no such table: Х"; hintSchemaError(other) != other {
		t.Errorf("прочие ошибки не меняем, got %q", hintSchemaError(other))
	}
}

func TestParseWidgetYAML(t *testing.T) {
	w, err := parseWidgetYAML("name: Тест\ntype: chart\nquery: ВЫБРАТЬ 1\n")
	if err != nil {
		t.Fatalf("валидный YAML не разобрался: %v", err)
	}
	if w.Name != "Тест" || w.Type != metadata.WidgetTypeChart {
		t.Errorf("разобрано неверно: %+v", w)
	}
	if w.ChartKind != "bar" {
		t.Errorf("дефолт chart_kind должен быть bar, got %q", w.ChartKind)
	}
	if _, err := parseWidgetYAML("type: chart\n"); err == nil {
		t.Error("YAML без name должен дать ошибку")
	}
}
