package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/auth"
)

func TestLoginJSONSetsSessionCookieAndDoesNotReturnToken(t *testing.T) {
	repo, ctx := newTestRepo(t)
	if _, err := repo.Create(ctx, "ivan", "secret123", "Иван", false); err != nil {
		t.Fatalf("Create: %v", err)
	}
	h := &auth.Handlers{Repo: repo}

	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"login":"ivan","password":"secret123"}`))
	req.RemoteAddr = "10.0.0.1:55555"
	rec := httptest.NewRecorder()
	h.LoginJSON(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("LoginJSON status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "onebase_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("LoginJSON did not set onebase_session cookie")
	}
	if !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode || sessionCookie.Path != "/" {
		t.Fatalf("unexpected session cookie attrs: %+v", sessionCookie)
	}
	if _, err := repo.LookupSession(ctx, sessionCookie.Value); err != nil {
		t.Fatalf("cookie session token is not valid: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["token"]; ok {
		t.Fatalf("LoginJSON must not expose session token in JSON body: %v", body)
	}
	user, _ := body["user"].(map[string]any)
	if user["login"] != "ivan" {
		t.Fatalf("unexpected response user: %v", body["user"])
	}
}
