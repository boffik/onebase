package websec

// Тесты security-заголовков и CSRF-защиты (план 53, этап 3; анализ §2.5).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestSecurityHeaders_Present(t *testing.T) {
	h := SecurityHeaders(okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/ui", nil))

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("нет Content-Security-Policy")
	}
	// база встраивается в iframe конфигуратора (другой порт = другой origin),
	// поэтому frame-ancestors должен допускать localhost, но не произвольный сайт
	for _, want := range []string{"frame-ancestors", "'self'", "http://localhost:*"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q не содержит %q", csp, want)
		}
	}
}

func csrfReq(method, origin, secFetchSite string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://localhost:9000/ui/docs/save", nil)
	req.Host = "localhost:9000"
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if secFetchSite != "" {
		req.Header.Set("Sec-Fetch-Site", secFetchSite)
	}
	rec := httptest.NewRecorder()
	CSRFProtect(okHandler()).ServeHTTP(rec, req)
	return rec
}

func TestCSRF_CrossOriginPOSTRejected(t *testing.T) {
	if rec := csrfReq("POST", "http://evil.example", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST: ожидался 403, получен %d", rec.Code)
	}
}

func TestCSRF_SameOriginPOSTAllowed(t *testing.T) {
	if rec := csrfReq("POST", "http://localhost:9000", ""); rec.Code != http.StatusOK {
		t.Fatalf("same-origin POST: ожидался 200, получен %d", rec.Code)
	}
}

func TestCSRF_NullOriginRejected(t *testing.T) {
	// Origin: null шлют sandboxed iframe и некоторые redirect-цепочки
	if rec := csrfReq("POST", "null", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("Origin:null POST: ожидался 403, получен %d", rec.Code)
	}
}

func TestCSRF_NoOriginNonBrowserAllowed(t *testing.T) {
	// curl/скрипты не шлют Origin и Sec-Fetch-Site — REST-клиенты не ломаются
	if rec := csrfReq("POST", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("POST без Origin (не браузер): ожидался 200, получен %d", rec.Code)
	}
}

func TestCSRF_SecFetchCrossSiteRejected(t *testing.T) {
	if rec := csrfReq("POST", "", "cross-site"); rec.Code != http.StatusForbidden {
		t.Fatalf("Sec-Fetch-Site:cross-site POST: ожидался 403, получен %d", rec.Code)
	}
}

func TestCSRF_GETNotChecked(t *testing.T) {
	if rec := csrfReq("GET", "http://evil.example", "cross-site"); rec.Code != http.StatusOK {
		t.Fatalf("GET не мутирует и не должен блокироваться, получен %d", rec.Code)
	}
}
