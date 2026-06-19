package launcher

import (
	"bytes"
	"strings"
	"testing"
)

// TestConfigurator_XSS_Escaped: имена объектов конфигурации экранируются и в
// HTML-, и в JS-контексте (план 55 этап 2 — закрывает XSS-долг плана 47 §1.3).
// До перехода на html/template падает: text/template вставляет payload сырым.
func TestConfigurator_XSS_Escaped(t *testing.T) {
	const payload = `<img src=x onerror=alert(1)>`
	data := &configuratorData{
		Base:           &Base{ID: "b", Name: "Тест", ConfigSource: "file"},
		Lang:           "ru",
		Tab:            "tree",
		Catalogs:       []cfgEntity{{Name: payload, Kind: "Справочник"}},
		AllEntityNames: []string{payload},
	}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "cfg-main", data); err != nil {
		t.Fatalf("ExecuteTemplate cfg-main: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, payload) {
		t.Fatal("XSS-payload попал в вывод неэкранированным")
	}
	if !strings.Contains(out, "&lt;img") && !strings.Contains(out, `<img`) {
		t.Fatal("ожидалась экранированная форма payload (HTML &lt; или JS \\u003c) — её нет")
	}
}
