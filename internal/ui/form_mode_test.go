package ui

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/storage"
	"github.com/ivantit66/onebase/internal/widget"
)

// newServerForFormMode конструирует минимальный *Server для тестов режима
// открытия форм (мирроринг TestAgentSettings_ForbiddenForNonAdmin: реестр +
// SQLite, но с миграцией и виджет-кэшем, чтобы прошёл полный рендер index).
func newServerForFormMode(t *testing.T) *Server {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, nil); err != nil {
		t.Fatal(err)
	}
	return &Server{
		store:       db,
		reg:         runtime.NewRegistry(),
		messages:    NewMessageStore(),
		widgetCache: widget.NewCache(60 * time.Second),
	}
}

func TestIndex_RedirectsToTabsWhenTabsMode(t *testing.T) {
	s := newServerForFormMode(t)
	ctx := context.Background()
	if err := s.store.SaveFormOpenMode(ctx, storage.FormModeTabs); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	// ASCII-имя подсистемы: http.Redirect percent-кодирует не-ASCII байты в
	// Location (стандартная библиотека), поэтому для точного сравнения строки
	// берём ASCII; кириллица сохраняется тем же образом, только в %xx-форме.
	req := httptest.NewRequest(http.MethodGet, "/ui?subsystem=Sales", nil)
	s.index(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ожидался 303, получено %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/ui/app?subsystem=Sales" {
		t.Errorf("Location = %q, ожидалось /ui/app?subsystem=Sales", loc)
	}
}

func TestIndex_NoRedirectInPagesMode(t *testing.T) {
	s := newServerForFormMode(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	s.index(rec, req)
	if rec.Code == http.StatusSeeOther {
		t.Fatalf("в режиме pages редиректа быть не должно, получено %d", rec.Code)
	}
}

func TestSetFormMode_PersistsAndRedirects(t *testing.T) {
	s := newServerForFormMode(t)
	rec := httptest.NewRecorder()
	// Персональный режим хранится по логину, поэтому привязываем пользователя к
	// запросу (анонимная сессия персонального режима не имеет — см. спеку).
	req := httptest.NewRequest(http.MethodPost, "/ui/form-mode", strings.NewReader("mode=tabs"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.ContextWithUser(req.Context(), &auth.User{Login: "ivan"}))
	s.setFormMode(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ожидался 303, получено %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/ui/app" {
		t.Errorf("Location = %q, ожидалось /ui/app", loc)
	}
	// Персональный режим действительно сохранён — последующее разрешение даёт tabs.
	if got := s.store.EffectiveFormOpenMode(req.Context(), "ivan"); got != storage.FormModeTabs {
		t.Errorf("после POST персональный режим не сохранён: %q", got)
	}
}

// TestSetFormMode_AnonymousFallsBackToGlobal проверяет краевой случай спеки:
// у анонимной сессии персонального режима нет, действует глобальный дефолт.
func TestSetFormMode_AnonymousFallsBackToGlobal(t *testing.T) {
	s := newServerForFormMode(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/form-mode", strings.NewReader("mode=tabs"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.setFormMode(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ожидался 303, получено %d", rec.Code)
	}
	// Глобально pages (дефолт), персональный не сохранён → редирект на /ui.
	if loc := rec.Header().Get("Location"); loc != "/ui" {
		t.Errorf("Location = %q, ожидалось /ui", loc)
	}
}

func renderNavWithMode(t *testing.T, mode string) string {
	t.Helper()
	var buf bytes.Buffer
	data := map[string]any{"Cfg": Config{}, "Lang": "ru", "IsAdmin": false, "FormOpenMode": mode}
	if err := tmpl.ExecuteTemplate(&buf, "nav", data); err != nil {
		t.Fatalf("execute nav: %v", err)
	}
	return buf.String()
}

func TestNav_ShowsTabsToggle(t *testing.T) {
	out := renderNavWithMode(t, storage.FormModePages)
	if !strings.Contains(out, "/ui/form-mode") {
		t.Error("в топбаре нет формы переключателя режима (/ui/form-mode)")
	}
}
