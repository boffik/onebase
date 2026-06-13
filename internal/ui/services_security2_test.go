package ui

// Юнит-тесты тира безопасности HTTP-сервисов и dev-инструментов (#5–#7).

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/httpservice"
)

// #5: safeJoinWithin не выпускает путь за пределы base (path traversal).
func TestSafeJoinWithin(t *testing.T) {
	base := filepath.Join("tmp", "gengen-xyz")
	ok := []string{"documents/заказ.yaml", "a/b/c.yaml", "file.os"}
	for _, rel := range ok {
		if _, err := safeJoinWithin(base, rel); err != nil {
			t.Errorf("safeJoinWithin(%q) неожиданно отклонён: %v", rel, err)
		}
	}
	bad := []string{
		"../escape.txt",
		"../../etc/passwd",
		"a/../../escape",
		`..\..\windows\startup.bat`,
	}
	for _, rel := range bad {
		if _, err := safeJoinWithin(base, rel); err == nil {
			t.Errorf("safeJoinWithin(%q) должен был отклонить выход за base", rel)
		}
	}
}

// #6: sessionToken берёт токен только из cookie, query-параметр _tk игнорируется
// (план 53 — токен в URL утекает в историю/логи).
func TestSessionToken_IgnoresQueryTk(t *testing.T) {
	// _tk в query больше не принимается
	r := httptest.NewRequest("GET", "/hs/x/?_tk=leaked-token", nil)
	if got := sessionToken(r); got != "" {
		t.Errorf("sessionToken не должен брать токен из ?_tk= (получено %q)", got)
	}
	// cookie по-прежнему работает
	r = httptest.NewRequest("GET", "/hs/x/", nil)
	r.AddCookie(&http.Cookie{Name: "onebase_session", Value: "cookie-token"})
	if got := sessionToken(r); got != "cookie-token" {
		t.Errorf("sessionToken должен брать токен из cookie, получено %q", got)
	}
}

// #7: matchOrigin различает явное совпадение и wildcard; setCORSHeaders не
// отражает произвольный Origin с Allow-Credentials при origins:["*"].
func TestMatchOrigin_WildcardFlag(t *testing.T) {
	if allow, wc, ok := matchOrigin([]string{"*"}, "https://evil.test"); !ok || !wc || allow != "*" {
		t.Errorf(`matchOrigin(["*"], origin) = (%q,%v,%v), ожидалось ("*",true,true)`, allow, wc, ok)
	}
	if allow, wc, ok := matchOrigin([]string{"https://shop.test"}, "https://shop.test"); !ok || wc || allow != "https://shop.test" {
		t.Errorf("явное совпадение должно давать (origin,false,true), получено (%q,%v,%v)", allow, wc, ok)
	}
	if _, _, ok := matchOrigin([]string{"https://shop.test"}, "https://evil.test"); ok {
		t.Error("чужой origin без wildcard не должен совпадать")
	}
}

func TestSetCORSHeaders_WildcardCredentialsNotReflected(t *testing.T) {
	s := &Server{}
	svc := &httpservice.Service{
		Name: "X", RootURL: "x",
		CORS: &httpservice.CORSConfig{Origins: []string{"*"}, Credentials: true},
	}
	r := httptest.NewRequest("GET", "/hs/x/", nil)
	r.Header.Set("Origin", "https://evil.test")
	w := httptest.NewRecorder()
	s.setCORSHeaders(w, r, svc)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("при origins:[*]+credentials произвольный Origin не должен отражаться, получено ACAO=%q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials не должен ставиться для wildcard-источника, получено %q", got)
	}
}

// Явно разрешённый источник с credentials — отражается (легитимный кейс).
func TestSetCORSHeaders_ExplicitCredentialsReflected(t *testing.T) {
	s := &Server{}
	svc := &httpservice.Service{
		Name: "X", RootURL: "x",
		CORS: &httpservice.CORSConfig{Origins: []string{"https://shop.test"}, Credentials: true},
	}
	r := httptest.NewRequest("GET", "/hs/x/", nil)
	r.Header.Set("Origin", "https://shop.test")
	w := httptest.NewRecorder()
	s.setCORSHeaders(w, r, svc)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://shop.test" {
		t.Errorf("явный источник должен отражаться, ACAO=%q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("для явного источника Allow-Credentials=true, получено %q", got)
	}
}

// #11: тело больше лимита → 413, а не молчаливое усечение.
func TestService_BodyTooLargeReturns413(t *testing.T) {
	s := newSecuredServiceServer(t, &httpservice.Service{
		Name: "T", RootURL: "t", Auth: "none",
		Templates: []httpservice.URLTemplate{{Template: "/echo", Methods: map[string]string{"POST": "Эхо"}}},
	})
	big := strings.NewReader(strings.Repeat("a", 2<<20)) // 2 МБ > 1 МБ лимит харнесса
	r := httptest.NewRequest("POST", "/hs/t/echo", big)
	w := httptest.NewRecorder()
	s.serviceDispatch(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("тело > лимита должно дать 413, получено %d", w.Code)
	}
}

// #10: мутирующий запрос с чужим Origin без CORS отклоняется (CSRF-эквивалент),
// а server-to-server без Origin проходит.
func TestService_CrossOriginMutatingRejected(t *testing.T) {
	s := newSecuredServiceServer(t, &httpservice.Service{
		Name: "T", RootURL: "t", Auth: "none",
		Templates: []httpservice.URLTemplate{{Template: "/echo", Methods: map[string]string{"POST": "Эхо"}}},
	})
	r := httptest.NewRequest("POST", "/hs/t/echo", strings.NewReader("{}"))
	r.Host = "localhost:8080"
	r.Header.Set("Origin", "https://evil.test")
	w := httptest.NewRecorder()
	s.serviceDispatch(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin мутирующий запрос без CORS → 403, получено %d", w.Code)
	}

	r = httptest.NewRequest("POST", "/hs/t/echo", strings.NewReader("{}"))
	w = httptest.NewRecorder()
	s.serviceDispatch(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("server-to-server POST без Origin должен пройти, получено %d (%s)", w.Code, w.Body.String())
	}
}

// #8: basic-auth сервиса троттлится после серии неудач (брутфорс-защита).
func TestService_BasicAuthBruteForceThrottled(t *testing.T) {
	s := newSecuredServiceServer(t, &httpservice.Service{
		Name: "T", RootURL: "t", Auth: "basic",
		Templates: []httpservice.URLTemplate{{Template: "/", Methods: map[string]string{"GET": "Корень"}}},
	})
	send := func() int {
		r := httptest.NewRequest("GET", "/hs/t/", nil)
		r.SetBasicAuth("admin", "wrong")
		w := httptest.NewRecorder()
		s.serviceDispatch(w, r)
		return w.Code
	}
	for i := 0; i < 5; i++ {
		if code := send(); code != http.StatusUnauthorized {
			t.Fatalf("попытка %d: ожидался 401, получен %d", i+1, code)
		}
	}
	if code := send(); code != http.StatusTooManyRequests {
		t.Fatalf("6-я попытка должна блокироваться (429), получен %d", code)
	}
}
