package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestStaticUIJS(t *testing.T) {
	r := chi.NewRouter()
	mountStatic(r)

	req := httptest.NewRequest(http.MethodGet, "/static/ui.js", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("/static/ui.js status = %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/javascript") {
		t.Fatalf("/static/ui.js content-type = %q", ct)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"window.obOpenInShell",
		"openRefPicker",
		"obInitMappedCharts",
		"window.rsBeforeSubmit",
		"data-ob-attachments",
		"window.onebaseDevice",
		"onebase:звонок.входящий",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/static/ui.js не содержит %q", want)
		}
	}
}
