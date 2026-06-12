package ui

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newRedirectReq собирает запрос с chi-route-параметрами для redirectDSLPrint
// и возвращает записанный ответ.
func newRedirectReq(path string) *httptest.ResponseRecorder {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("kind", "documents")
	rctx.URLParams.Add("entity", "sale")
	rctx.URLParams.Add("id", "00000000-0000-0000-0000-000000000001")
	rctx.URLParams.Add("pfName", "upd")

	req := httptest.NewRequest("GET", path, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	s := &Server{}
	s.redirectDSLPrint(rec, req)
	return rec
}

// TestRedirectDSLPrintKeepsQuery: 301 со старого /print-dsl/ должен сохранять
// строку запроса (минор-фикс плана 64, этап 3).
func TestRedirectDSLPrintKeepsQuery(t *testing.T) {
	rec := newRedirectReq("/ui/documents/sale/00000000-0000-0000-0000-000000000001/print-dsl/upd?form=upd&x=1")
	if rec.Code != 301 {
		t.Fatalf("status = %d (want 301)", rec.Code)
	}
	loc := rec.Header().Get("Location")
	want := "/ui/documents/sale/00000000-0000-0000-0000-000000000001/print/upd?form=upd&x=1"
	if loc != want {
		t.Fatalf("Location = %q\nwant     %q", loc, want)
	}
}

// TestRedirectDSLPrintNoQuery: без query строка запроса не приклеивается
// (нет висящего «?»).
func TestRedirectDSLPrintNoQuery(t *testing.T) {
	rec := newRedirectReq("/ui/documents/sale/00000000-0000-0000-0000-000000000001/print-dsl/upd")
	loc := rec.Header().Get("Location")
	want := "/ui/documents/sale/00000000-0000-0000-0000-000000000001/print/upd"
	if loc != want {
		t.Fatalf("Location = %q\nwant     %q", loc, want)
	}
}

// TestRedirectDSLPrintPDFKeepsQuery: PDF-хвост и query сохраняются одновременно.
func TestRedirectDSLPrintPDFKeepsQuery(t *testing.T) {
	rec := newRedirectReq("/ui/documents/sale/00000000-0000-0000-0000-000000000001/print-dsl/upd/pdf?form=upd")
	loc := rec.Header().Get("Location")
	want := "/ui/documents/sale/00000000-0000-0000-0000-000000000001/print/upd/pdf?form=upd"
	if loc != want {
		t.Fatalf("Location = %q\nwant     %q", loc, want)
	}
}
