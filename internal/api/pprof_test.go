package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func pprofTestRouter(token string) *chi.Mux {
	r := chi.NewRouter()
	mountPprof(r, token)
	return r
}

func TestPprof_RequiresToken(t *testing.T) {
	r := pprofTestRouter("s3cr3t")

	// Без токена — 401.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("без токена ожидался 401, получили %d", rec.Code)
	}

	// Неверный токен — 401.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.Header.Set("X-OneBase-Debug-Token", "wrong")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("с неверным токеном ожидался 401, получили %d", rec.Code)
	}
}

func TestPprof_TokenViaHeaderAndQuery(t *testing.T) {
	r := pprofTestRouter("s3cr3t")

	// Токен в заголовке.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.Header.Set("X-OneBase-Debug-Token", "s3cr3t")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("с токеном в заголовке ожидался 200, получили %d", rec.Code)
	}

	// Токен в query — нужен для `go tool pprof …?token=…`.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/?token=s3cr3t", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("с токеном в query ожидался 200, получили %d", rec.Code)
	}

	// Именованный профиль (goroutine) обслуживается pprof.Index по /{name}.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/goroutine?token=s3cr3t&debug=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("для goroutine-профиля ожидался 200, получили %d", rec.Code)
	}
}

// Токен не задан (пустой) — гейт всегда закрыт, даже на пустой токен в запросе.
func TestPprof_EmptyTokenAlwaysDenied(t *testing.T) {
	r := pprofTestRouter("")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/?token=", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("при пустом серверном токене ожидался 401, получили %d", rec.Code)
	}
}
