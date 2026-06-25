package ui

import (
	"bytes"
	"strings"
	"testing"
)

// Оболочка вкладок (issue #129/#130, фаза 1) должна рендериться, переиспользуя
// head+nav, и содержать полосу вкладок + движок.
func TestAppShell_Render(t *testing.T) {
	data := map[string]any{
		"Cfg":              Config{AppName: "Test"},
		"Lang":             "ru",
		"Subsystems":       []any{},
		"CurrentSubsystem": "",
		"Nav":              []any{},
		"CollapsibleNav":   false,
		"IsAdmin":          false,
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "page-app-shell", data); err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`id="ob-tabstrip"`,         // полоса вкладок
		`id="ob-tabbody"`,          // область фреймов
		`class="ob-shell-main"`,    // контент вместо <main>
		`window.obOpenTab=openTab`, // движок вкладок
		`'obTabs'`,                 // ключ sessionStorage для restore
		`source==='obOpenTab'`,     // приём запросов из iframe
		`<header class="topbar">`,  // переиспользован nav (хром оболочки)
		`ob-tab-dup`,               // кнопка «новый экземпляр» (#130)
		`{allowDup:true}`,          // дубликат = новый экземпляр
		`source==='obDirty'`,       // фаза 3: приём флага несохранённых правок
		`tabByWindow`,              // маршрутизация по окну-источнику
		`beforeunload`,             // предупреждение при уходе со страницы
		`ob-tabmenu`,               // фаза 4: контекст-меню вкладки
		`Закрыть другие`,           // пункт контекст-меню
		`scrollIntoView`,           // автоскролл активной вкладки
	} {
		if !strings.Contains(html, want) {
			t.Errorf("оболочка вкладок не содержит %q", want)
		}
	}
}

// В партиале head есть детект встраивания и правило скрытия хрома (фаза 1):
// любая страница во фрейме оболочки прячет топбар/подсистемы.
func TestHead_EmbeddedChromeHidden(t *testing.T) {
	data := map[string]any{"Cfg": Config{AppName: "Test"}, "Lang": "ru"}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "head", data); err != nil {
		t.Fatalf("ExecuteTemplate head: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`window.__obEmbedded = (window.self !== window.top)`,
		`ob-embedded`,
		`.ob-embedded .topbar,.ob-embedded .subsys-bar,.ob-embedded #ob-nav{display:none`,
		`obOpenableForm`,                           // фаза 2: перехват открытия форм во вкладку
		`window.obOpenInShell`,                     // общий helper для ссылок и JS-открытий списков
		`source: 'obOpenTab'`,                      // постит запрос родителю-оболочке
		`window.parent && window.parent.obOpenTab`, // guard: только если родитель — оболочка
		`source: 'obDirty'`,                        // фаза 3: трекер несохранённых правок
	} {
		if !strings.Contains(html, want) {
			t.Errorf("head не содержит embedded-логику %q", want)
		}
	}
}
