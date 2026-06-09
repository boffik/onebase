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
