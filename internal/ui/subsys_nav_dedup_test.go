package ui

// #215 п.2: в панели подсистем хардкодом печатается ведущая ссылка «Главная», а
// затем перебираются подсистемы. Если в базе есть подсистема с представлением
// «Главная», она дублировала ведущую ссылку (видно справа от неё). Дедуп в шаблоне
// nav пропускает подсистему, чьё DisplayName совпадает с меткой домашней ссылки.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

func renderNavSubs(t *testing.T, subs []*metadata.Subsystem) string {
	t.Helper()
	data := map[string]any{
		"Cfg":              Config{},
		"Lang":             "ru",
		"IsAdmin":          false,
		"Subsystems":       subs,
		"CurrentSubsystem": "",
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "nav", data); err != nil {
		t.Fatalf("execute nav: %v", err)
	}
	return buf.String()
}

func TestSubsysBar_DedupHomeSubsystem(t *testing.T) {
	out := renderNavSubs(t, []*metadata.Subsystem{
		{Name: "Главная", Title: "Главная"},
		{Name: "Продажи", Title: "Продажи"},
	})
	// Единственная ссылка-подсистема в панели — «Продажи»; «Главная» как
	// подсистема не печатается (ведущая ссылка её уже представляет).
	if n := strings.Count(out, "?subsystem="); n != 1 {
		t.Errorf("ожидалась 1 ссылка-подсистема (Продажи), нашли %d:\n%s", n, out)
	}
	if !strings.Contains(out, "Продажи") {
		t.Errorf("обычная подсистема «Продажи» должна остаться в панели:\n%s", out)
	}
}

func TestSubsysBar_KeepsNonHomeSubsystems(t *testing.T) {
	out := renderNavSubs(t, []*metadata.Subsystem{
		{Name: "Продажи", Title: "Продажи"},
		{Name: "Закупки", Title: "Закупки"},
	})
	if n := strings.Count(out, "?subsystem="); n != 2 {
		t.Errorf("обе обычные подсистемы должны остаться (2 ссылки), нашли %d:\n%s", n, out)
	}
}

// Дедуп опирается на представление: подсистема с Name=Главная, но другим
// заголовком (Title) — не дубль и должна остаться.
func TestSubsysBar_DedupByDisplayNameNotRawName(t *testing.T) {
	out := renderNavSubs(t, []*metadata.Subsystem{
		{Name: "Главная", Title: "Основное"},
	})
	if n := strings.Count(out, "?subsystem="); n != 1 {
		t.Errorf("подсистема с заголовком «Основное» не дубль — должна остаться, нашли %d ссылок", n)
	}
}
