package ui

import (
	"bytes"
	"strings"
	"testing"

	reportpkg "github.com/ivantit66/onebase/internal/report"
)

func TestEffectiveComposition(t *testing.T) {
	main := &reportpkg.Composition{Groupings: []string{"Основной"}}
	variantComp := &reportpkg.Composition{Groupings: []string{"Вариант"}}
	override := &reportpkg.Composition{Groupings: []string{"Override"}}
	rep := &reportpkg.Report{
		Composition: main,
		Variants:    []reportpkg.ReportVariant{{Name: "V", Composition: variantComp}},
	}

	// 1) override (settings.Composition) перекрывает и вариант, и основной.
	if got := effectiveComposition(rep, &reportpkg.UserReportSettings{Variant: "V", Composition: override}); got != override {
		t.Fatalf("override: %+v", got)
	}
	// 2) settings без Composition → активный вариант по имени.
	if got := effectiveComposition(rep, &reportpkg.UserReportSettings{Variant: "V"}); got != variantComp {
		t.Fatalf("variant: %+v", got)
	}
	// 3) settings == nil → основной composition.
	if got := effectiveComposition(rep, nil); got != main {
		t.Fatalf("main: %+v", got)
	}
}

// TestReportSettingsPanel: панель «Настройки» рендерится при наличии ReportCols,
// содержит чекбоксы доступных полей и скрытое поле __settings.
func TestReportSettingsPanel(t *testing.T) {
	rep := &reportpkg.Report{Name: "sales", Title: "Продажи"}
	var buf bytes.Buffer
	data := map[string]any{
		"Report":       rep,
		"ParamValues":  map[string]any{},
		"ReportParams": []reportParamUI{},
		"ReportCols":   []string{"Товар", "Сумма"},
		"UserSettings": &reportpkg.UserReportSettings{
			Composition: &reportpkg.Composition{
				Groupings: []string{"Товар"},
				Measures:  []reportpkg.Measure{{Field: "Сумма", Agg: "sum"}},
			},
		},
		"Cfg":  Config{},
		"Lang": "ru",
	}
	if err := tmpl.ExecuteTemplate(&buf, "page-report", data); err != nil {
		t.Fatalf("execute page-report: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `data-block="settings"`) {
		t.Fatalf("нет блока настроек data-block=settings")
	}
	if !strings.Contains(out, `name="__settings"`) {
		t.Fatalf("нет скрытого поля __settings")
	}
	for _, want := range []string{"Товар", "Сумма"} {
		if !strings.Contains(out, want) {
			t.Errorf("в панели нет поля %q", want)
		}
	}
}

// TestReportSettingsPanelHidden: без ReportCols панель не рендерится
// (обратная совместимость с отчётами без компоновки).
func TestReportSettingsPanelHidden(t *testing.T) {
	rep := &reportpkg.Report{Name: "plain", Title: "Простой"}
	var buf bytes.Buffer
	data := map[string]any{
		"Report":       rep,
		"ParamValues":  map[string]any{},
		"ReportParams": []reportParamUI{},
		"Cfg":          Config{},
		"Lang":         "ru",
	}
	if err := tmpl.ExecuteTemplate(&buf, "page-report", data); err != nil {
		t.Fatalf("execute page-report: %v", err)
	}
	if strings.Contains(buf.String(), `data-block="settings"`) {
		t.Errorf("панель настроек не должна рендериться без ReportCols")
	}
}
