package launcher

import (
	"bytes"
	"strings"
	"testing"
)

// Issue #35: редактор объектов конфигуратора рендерит секции вкладками,
// а не длинным столбцом. Фиксируем наличие панели вкладок и панелей контента.
func renderTabTree(t *testing.T) string {
	t.Helper()
	data := &configuratorData{
		Base: &Base{ID: "test-base", Name: "Тест", ConfigSource: "file"},
		Lang: "ru",
		Tab:  "tree",
		Catalogs: []cfgEntity{{
			Name: "Номенклатура", Kind: "Справочник",
			Fields: []cfgField{{Name: "Цена", Type: "number"}},
		}},
		Docs: []cfgEntity{{
			Name: "Реализация", Kind: "Документ", Posting: true,
			Fields: []cfgField{{Name: "Сумма", Type: "number"}},
		}},
		Reports:    []cfgReport{{Name: "Продажи"}},
		Processors: []cfgProcessor{{Name: "Загрузка"}},
	}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "tab-tree", data); err != nil {
		t.Fatalf("ExecuteTemplate tab-tree: %v", err)
	}
	return buf.String()
}

func TestConfigurator_EntityTabs(t *testing.T) {
	html := renderTabTree(t)
	for _, sub := range []string{
		`class="obj-tabs"`,
		`id="ot-data-Номенклатура"`,
		`id="ot-forms-Номенклатура"`,
		`id="ot-print-Номенклатура"`,
		`id="ot-modules-Номенклатура"`,
		`id="ot-data-Реализация"`,
		`id="ot-modules-Реализация"`,
	} {
		if !strings.Contains(html, sub) {
			t.Errorf("в HTML нет ожидаемого фрагмента: %q", sub)
		}
	}
}

func TestConfigurator_ReportTabs(t *testing.T) {
	html := renderTabTree(t)
	for _, sub := range []string{
		`id="ot-params-Продажи"`,
		`id="ot-query-Продажи"`,
		`id="ot-chart-Продажи"`,
	} {
		if !strings.Contains(html, sub) {
			t.Errorf("в HTML нет ожидаемого фрагмента отчёта: %q", sub)
		}
	}
}

func TestConfigurator_ProcessorTabs(t *testing.T) {
	html := renderTabTree(t)
	for _, sub := range []string{
		`id="ot-params-Загрузка"`,
		`id="ot-code-Загрузка"`,
		`id="ot-form-Загрузка"`,
	} {
		if !strings.Contains(html, sub) {
			t.Errorf("в HTML нет ожидаемого фрагмента обработки: %q", sub)
		}
	}
}
