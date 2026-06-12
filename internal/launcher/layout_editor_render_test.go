package launcher

import (
	"bytes"
	"strings"
	"testing"
)

// План 64, этап 5a: визуальный редактор макетов на модели v2. Фиксируем наличие
// новых элементов панели редактора (HTML тоггл-кнопок границ по сторонам) и
// JS-функций (порядок областей, линейка колонок, высоты строк, границы).

// renderLayoutPanelTree рендерит tab-tree с одной DSL-печатной формой, у которой
// привязан макет, — чтобы попала панель редактора макета (mkt-*).
func renderLayoutPanelTree(t *testing.T) string {
	t.Helper()
	data := &configuratorData{
		Base: &Base{ID: "test-base", Name: "Тест", ConfigSource: "file"},
		Lang: "ru",
		Tab:  "tree",
		DSLPrintForms: []cfgDSLPrintForm{{
			Name:       "Накладная",
			Document:   "Реализация",
			Source:     "// форма",
			FileName:   "Накладная.os",
			HasLayout:  true,
			LayoutYAML: "areas:\n  - name: Заголовок\n    rows: []\n",
		}},
	}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "tab-tree", data); err != nil {
		t.Fatalf("ExecuteTemplate tab-tree: %v", err)
	}
	return buf.String()
}

// renderCfgFootJS рендерит cfg-foot (там живёт JS редактора макетов).
func renderCfgFootJS(t *testing.T) string {
	t.Helper()
	data := &configuratorData{Base: &Base{ID: "test-base", Name: "Тест", ConfigSource: "file"}, Lang: "ru"}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "cfg-foot", data); err != nil {
		t.Fatalf("ExecuteTemplate cfg-foot: %v", err)
	}
	return buf.String()
}

// 6.3: панель свойств ячейки содержит тоггл-кнопки границ по сторонам, select
// толщины и кнопки-пресеты.
func TestLayoutEditor_PerSideBorderControls(t *testing.T) {
	html := renderLayoutPanelTree(t)
	for _, sub := range []string{
		`id="vp-bd-left-Накладная"`,
		`id="vp-bd-top-Накладная"`,
		`id="vp-bd-right-Накладная"`,
		`id="vp-bd-bottom-Накладная"`,
		`id="vp-bw-Накладная"`,
		`ldToggleBorderSide('Накладная','left')`,
		`ldBorderPreset('Накладная','all')`,
		`ldBorderPreset('Накладная','none')`,
		`ldBorderGridArea('Накладная')`,
	} {
		if !strings.Contains(html, sub) {
			t.Errorf("в HTML панели редактора нет фрагмента: %q", sub)
		}
	}
}

// 6.1/6.2/6.3: JS-редактор содержит функции порядка областей, линейки колонок,
// высот строк и границ по сторонам.
func TestLayoutEditor_V2JSFunctions(t *testing.T) {
	js := renderCfgFootJS(t)
	for _, sub := range []string{
		"function moveLayoutArea",    // 6.1 порядок областей
		"function _ldNormAreas",      // 6.1 чтение map → массив
		"function ldColWidth",        // 6.2 ширины колонок
		"function ldRowHeight",       // 6.2 высоты строк
		"function _ldRuler",          // 6.2 линейка
		"function ldToggleBorderSide", // 6.3 границы по сторонам
		"function ldBorderPreset",    // 6.3 пресеты
		"function ldBorderGridArea",  // 6.3 сетка области
	} {
		if !strings.Contains(js, sub) {
			t.Errorf("в JS редактора нет функции: %q", sub)
		}
	}
}

// 5b блок A: операции многоуровневых шапок — удаление ячейки, вертикальный
// merge/unmerge, раскладка по канону модели, отказ %-ширин.
func TestLayoutEditor_StageBJSFunctions(t *testing.T) {
	js := renderCfgFootJS(t)
	for _, sub := range []string{
		"function ldDelCell",         // A.1 удаление одиночной ячейки
		"function ldMergeDown",       // A.2 вертикальный merge
		"function ldUnmergeVertical", // A.2 разъединение вниз
		"function _ldColLayout",      // канон раскладки спанов
		"function _ldVisualCol",
		"function _ldCellIndexAtCol",
	} {
		if !strings.Contains(js, sub) {
			t.Errorf("в JS редактора нет функции: %q", sub)
		}
	}
	// %-ширины отклоняются в ldColWidth.
	if !strings.Contains(js, "indexOf('%')") {
		t.Error("ldColWidth не отклоняет %-ширины")
	}
}

// 5b блок A: HTML панели редактора содержит кнопки удаления ячейки и
// вертикального merge/unmerge.
func TestLayoutEditor_StageBControls(t *testing.T) {
	html := renderLayoutPanelTree(t)
	for _, sub := range []string{
		`ldDelCell('Накладная')`,
		`ldMergeDown('Накладная')`,
		`ldUnmergeVertical('Накладная')`,
	} {
		if !strings.Contains(html, sub) {
			t.Errorf("в HTML панели редактора нет фрагмента: %q", sub)
		}
	}
}
