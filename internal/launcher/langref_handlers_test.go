package launcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestConfiguratorLangref_ReturnsJSON(t *testing.T) {
	s := newTestStore(t)
	b := &Base{Name: "Тест", DB: "postgres://localhost/x", Port: 8080}
	if err := s.Add(b); err != nil {
		t.Fatalf("Add: %v", err)
	}
	h := &handler{store: s}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", b.ID)
	req := httptest.NewRequest(http.MethodGet, "/bases/"+b.ID+"/configurator/langref", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.configuratorLangref(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, тело: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("ответ не JSON-массив: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("ожидался непустой справочник langref")
	}
}
