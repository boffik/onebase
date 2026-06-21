package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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
