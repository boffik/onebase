package launcher

import (
	"bytes"
	"strings"
	"testing"
)

func renderCfgFoot(t *testing.T) string {
	t.Helper()
	data := &configuratorData{Base: &Base{ID: "test-base", Name: "Тест", ConfigSource: "file"}, Lang: "ru"}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "cfg-foot", data); err != nil {
		t.Fatalf("ExecuteTemplate cfg-foot: %v", err)
	}
	return buf.String()
}

func TestConfigurator_LangrefWired(t *testing.T) {
	html := renderCfgFoot(t)
	for _, sub := range []string{
		"registerHoverProvider",
		"registerSignatureHelpProvider",
		"/configurator/langref",
		"function loadLangref",
	} {
		if !strings.Contains(html, sub) {
			t.Errorf("в cfg-foot нет ожидаемого фрагмента: %q", sub)
		}
	}
}

func TestConfigurator_SyntaxPanelWired(t *testing.T) {
	data := &configuratorData{Base: &Base{ID: "test-base", Name: "Тест", ConfigSource: "file"}, Lang: "ru", Tab: "tree"}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "cfg-main", data); err != nil {
		t.Fatalf("ExecuteTemplate cfg-main: %v", err)
	}
	html := buf.String()
	for _, sub := range []string{
		`id="syntax-ref-panel"`,
		`id="syntax-ref-toggle"`,
		"function toggleSyntaxRef",
		"function insertLangrefSignature",
	} {
		if !strings.Contains(html, sub) {
			t.Errorf("в cfg-main нет фрагмента окна-справочника: %q", sub)
		}
	}
}
