package launcher

import (
	"net/url"
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
