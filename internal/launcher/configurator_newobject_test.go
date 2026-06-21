package launcher

import (
	"strings"
	"testing"
)

// newObjectContent("module") должен отдавать файл src/<имя>.module.os со
// стартовой экспортной процедурой. Раньше ветки "module" не было — функция
// возвращала пустой subdir, и создание общего модуля из дерева конфигуратора
// падало с «Неизвестный тип объекта: module» (issue #95).
func TestNewModuleContent(t *testing.T) {
	subdir, content := newObjectContent("module", "ОбщийМодуль")
	if subdir != "src" {
		t.Errorf("subdir = %q, ожидался src", subdir)
	}
	if !strings.Contains(content, "Процедура") || !strings.Contains(content, "Экспорт") {
		t.Errorf("в скелете модуля нет экспортной процедуры: %q", content)
	}
}

// treeNodeID должен возвращать data-id узла дерева, совпадающий с разметкой
// configurator_tmpl.go, для каждого вида объекта (issue #127).
func TestTreeNodeID(t *testing.T) {
	cases := map[string]string{
		"catalog":    "e-Контрагент",
		"document":   "e-Контрагент",
		"register":   "r-Контрагент",
		"inforeg":    "ir-Контрагент",
		"accountreg": "ar-Контрагент",
		"enum":       "en-Контрагент",
		"subsystem":  "sub-Контрагент",
		"widget":     "wdg-Контрагент",
		"processor":  "proc-Контрагент",
		"page":       "page-Контрагент",
		"module":     "mod-Контрагент",
	}
	for kind, want := range cases {
		if got := treeNodeID(kind, "Контрагент"); got != want {
			t.Errorf("treeNodeID(%q) = %q, ожидалось %q", kind, got, want)
		}
	}
	if got := treeNodeID("неизвестный", "X"); got != "" {
		t.Errorf("неизвестный вид → ожидалась пустая строка, получено %q", got)
	}
}

// Инвариант: любой вид, который умеет создавать newObjectContent (непустой
// subdir), обязан иметь узел в дереве (непустой treeNodeID) — иначе после
// создания позиционироваться будет не на что (issue #127).
func TestTreeNodeID_CoversCreatableKinds(t *testing.T) {
	kinds := []string{"catalog", "document", "register", "inforeg", "enum", "subsystem", "widget", "accountreg", "processor", "page", "module"}
	for _, kind := range kinds {
		if subdir, _ := newObjectContent(kind, "Тест"); subdir == "" {
			t.Fatalf("newObjectContent(%q) вернул пустой subdir — обнови список видов в тесте", kind)
		}
		if treeNodeID(kind, "Тест") == "" {
			t.Errorf("вид %q создаётся, но treeNodeID пуст — узел дерева не подсветится после создания", kind)
		}
	}
}
