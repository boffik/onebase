package ui

// issue #304: config/home_page.yaml → hidden прячет глобальную «Главную».
// Ведущая ссылка «Главная» уходит из панели разделов, а вход (/ui/, /ui/app)
// уводит на первый раздел. Фейл-сейф: без подсистем прятать нечем.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/storage"
)

// loadHiddenHome помечает глобальную «Главную» скрытой и грузит подсистемы в
// заданном порядке (первый — цель редиректа).
func loadHiddenHome(s *Server, subs ...string) {
	var list []*metadata.Subsystem
	for _, n := range subs {
		list = append(list, &metadata.Subsystem{Name: n, Title: n})
	}
	s.reg.LoadSubsystems(list)
	s.reg.LoadHomePage(&metadata.HomePage{Hidden: true})
}

func TestIndex_HiddenHome_RedirectsToFirstSubsystem(t *testing.T) {
	s := newServerForFormMode(t)
	loadHiddenHome(s, "Sales", "Buys")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	s.index(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ожидался 303, получено %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/ui/?subsystem=Sales" {
		t.Errorf("Location = %q, ожидалось /ui/?subsystem=Sales", loc)
	}
}

func TestIndex_HiddenHome_TabsMode_RedirectsToAppShell(t *testing.T) {
	s := newServerForFormMode(t)
	if err := s.store.SaveFormOpenMode(context.Background(), storage.FormModeTabs); err != nil {
		t.Fatal(err)
	}
	loadHiddenHome(s, "Sales")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	s.index(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ожидался 303, получено %d", rec.Code)
	}
	// В режиме вкладок цель — оболочка, одним хопом (не /ui/?subsystem=…).
	if loc := rec.Header().Get("Location"); loc != "/ui/app?home=1&subsystem=Sales" {
		t.Errorf("Location = %q, ожидалось /ui/app?home=1&subsystem=Sales", loc)
	}
}

func TestAppShell_HiddenHome_RedirectsToFirstSubsystem(t *testing.T) {
	s := newServerForFormMode(t)
	loadHiddenHome(s, "Sales")
	rec := httptest.NewRecorder()
	// Прямой заход на скрытый стол оболочки без раздела тоже уводит на раздел.
	req := httptest.NewRequest(http.MethodGet, "/ui/app?home=1", nil)
	s.appShell(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ожидался 303, получено %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/ui/app?home=1&subsystem=Sales" {
		t.Errorf("Location = %q, ожидалось /ui/app?home=1&subsystem=Sales", loc)
	}
}

func TestIndex_HiddenHome_WithSubsystemParam_NoRedirect(t *testing.T) {
	s := newServerForFormMode(t)
	loadHiddenHome(s, "Sales")
	rec := httptest.NewRecorder()
	// Раздел выбран явно — показываем его, а не редиректим на «первый».
	req := httptest.NewRequest(http.MethodGet, "/ui/?subsystem=Sales", nil)
	s.index(rec, req)
	if rec.Code == http.StatusSeeOther {
		t.Fatalf("при заданном subsystem редиректа скрытия быть не должно, получен %d → %s",
			rec.Code, rec.Header().Get("Location"))
	}
}

func TestIndex_HiddenHome_NoSubsystems_NoRedirect(t *testing.T) {
	s := newServerForFormMode(t)
	// Скрыта, но подсистем нет → фейл-сейф: не редиректим, иначе навигации нет.
	s.reg.LoadHomePage(&metadata.HomePage{Hidden: true})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	s.index(rec, req)
	if rec.Code == http.StatusSeeOther {
		t.Fatalf("без подсистем скрытие не должно редиректить, получен %d → %s",
			rec.Code, rec.Header().Get("Location"))
	}
}

func renderNavHideHome(t *testing.T, hide bool, subs []*metadata.Subsystem) string {
	t.Helper()
	data := map[string]any{
		"Cfg":              Config{},
		"Lang":             "ru",
		"IsAdmin":          false,
		"Subsystems":       subs,
		"CurrentSubsystem": "",
		"HideHome":         hide,
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "nav", data); err != nil {
		t.Fatalf("execute nav: %v", err)
	}
	return buf.String()
}

// leadHomeLink — маркер ведущей ссылки «Главная» в панели разделов (класс active
// при отсутствии активного раздела). У заголовка топбара href="/ui/" другой класс
// (topbar-title), поэтому маркер однозначен.
const leadHomeLink = `href="/ui/" class="active">`

func TestSubsysBar_HideHome_OmitsLeadingHomeLink(t *testing.T) {
	subs := []*metadata.Subsystem{{Name: "Sales", Title: "Sales"}}

	shown := renderNavHideHome(t, false, subs)
	if !strings.Contains(shown, leadHomeLink) {
		t.Errorf("при видимой Главной ведущая ссылка должна быть:\n%s", shown)
	}

	hidden := renderNavHideHome(t, true, subs)
	if strings.Contains(hidden, leadHomeLink) {
		t.Errorf("при HideHome ведущей ссылки «Главная» быть не должно:\n%s", hidden)
	}
	if !strings.Contains(hidden, "?subsystem=Sales") {
		t.Errorf("раздел Sales должен остаться в панели:\n%s", hidden)
	}
}

// При HideHome одноимённый раздел «Главная» больше не дедупится (ведущей ссылки,
// которую он дублировал бы, нет) — он должен показываться как обычный раздел.
func TestSubsysBar_HideHome_KeepsHomeNamedSubsystem(t *testing.T) {
	subs := []*metadata.Subsystem{{Name: "Главная", Title: "Главная"}}
	out := renderNavHideHome(t, true, subs)
	if !strings.Contains(out, "?subsystem=") {
		t.Errorf("одноимённый раздел «Главная» при HideHome должен показываться:\n%s", out)
	}
}
