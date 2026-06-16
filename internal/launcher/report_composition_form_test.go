package launcher

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseCompositionForm(t *testing.T) {
	f := url.Values{}
	f.Set("comp.present", "1")
	f.Set("comp.grouping.0", "Менеджер")
	f.Set("comp.grouping.1", "Клиент")
	f.Set("comp.measure.0.field", "Сумма")
	f.Set("comp.measure.0.agg", "sum")
	f.Set("comp.measure.0.title", "Сумма, ₽")
	f.Set("comp.totals.grand", "on")
	f.Set("comp.totals.subtotals", "on")
	f.Set("comp.detail", "on")
	f.Set("comp.sort.0.field", "Сумма")
	f.Set("comp.sort.0.dir", "desc")
	f.Set("comp.cond.0.when", "Сумма < 0")
	f.Set("comp.cond.0.color", "#c00")
	f.Set("comp.cond.0.bold", "on")
	f.Set("comp.chart.type", "bar")
	f.Set("comp.chart.category", "Менеджер")
	f.Set("comp.chart.series", "Сумма")

	c, present := parseCompositionForm(f)
	if !present {
		t.Fatal("present=false")
	}
	if c == nil {
		t.Fatal("composition nil")
	}
	if len(c.Groupings) != 2 || c.Groupings[1] != "Клиент" {
		t.Fatalf("groupings: %v", c.Groupings)
	}
	if len(c.Measures) != 1 || c.Measures[0].Agg != "sum" || c.Measures[0].Title != "Сумма, ₽" {
		t.Fatalf("measures: %+v", c.Measures)
	}
	if !c.Totals.Grand || !c.Totals.Subtotals || !c.Detail {
		t.Fatalf("totals/detail")
	}
	if len(c.Sort) != 1 || c.Sort[0].Dir != "desc" {
		t.Fatalf("sort: %+v", c.Sort)
	}
	if len(c.Conditional) != 1 || c.Conditional[0].Style.Color != "#c00" || !c.Conditional[0].Style.Bold {
		t.Fatalf("cond: %+v", c.Conditional)
	}
	if c.Chart == nil || c.Chart.Type != "bar" || c.Chart.Category != "Менеджер" || len(c.Chart.Series) != 1 {
		t.Fatalf("chart: %+v", c.Chart)
	}
}

func TestParseCompositionFormDefaultColors(t *testing.T) {
	f := url.Values{}
	f.Set("comp.present", "1")
	f.Set("comp.grouping.0", "М")
	f.Set("comp.cond.0.when", "X > 0")
	f.Set("comp.cond.0.color", "#000000")      // дефолт → пусто
	f.Set("comp.cond.0.background", "#ffffff") // дефолт → пусто
	f.Set("comp.cond.0.bold", "on")
	c, _ := parseCompositionForm(f)
	if c == nil || len(c.Conditional) != 1 {
		t.Fatalf("ожидали 1 правило: %+v", c)
	}
	s := c.Conditional[0].Style
	if s.Color != "" || s.Background != "" {
		t.Fatalf("дефолт-цвета должны стать пустыми: %+v", s)
	}
	if !s.Bold {
		t.Fatal("bold потерян")
	}
}

func TestParseCompositionFormAbsentAndEmpty(t *testing.T) {
	if c, present := parseCompositionForm(url.Values{}); present || c != nil {
		t.Fatalf("absent: present=%v c=%v", present, c)
	}
	f := url.Values{}
	f.Set("comp.present", "1")
	c, present := parseCompositionForm(f)
	if !present || c != nil {
		t.Fatalf("empty: present=%v c=%v (ждали present=true, c=nil)", present, c)
	}
}

func TestApplyReportComposition(t *testing.T) {
	raw := []byte("name: R\nquery: \"ВЫБРАТЬ 1\"\ncomposition:\n  groupings: [Старое]\n  measures:\n    - {field: X, agg: sum}\n")

	// форма без present → composition сохраняется как была
	out, err := applyReportComposition(raw, url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Старое") {
		t.Fatalf("composition должна сохраниться без present:\n%s", out)
	}

	// present + новые поля → перезапись
	f := url.Values{}
	f.Set("comp.present", "1")
	f.Set("comp.grouping.0", "Новое")
	f.Set("comp.measure.0.field", "Сумма")
	f.Set("comp.measure.0.agg", "sum")
	out, _ = applyReportComposition(raw, f)
	if !strings.Contains(string(out), "Новое") || strings.Contains(string(out), "Старое") {
		t.Fatalf("composition должна перезаписаться:\n%s", out)
	}

	// present, пусто → composition удаляется
	f2 := url.Values{}
	f2.Set("comp.present", "1")
	out, _ = applyReportComposition(raw, f2)
	if strings.Contains(string(out), "composition") {
		t.Fatalf("composition должна очиститься:\n%s", out)
	}
}
