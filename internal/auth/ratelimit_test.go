package auth_test

// Тесты rate-limiting логина (план 53, этап 2): брутфорс пароля ограничивается
// in-memory лимитером по ключу (IP, login).

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/auth"
)

func TestLoginLimiter_BlocksAfterMaxFails(t *testing.T) {
	l := auth.NewLoginLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("1.2.3.4|ivan"); !ok {
			t.Fatalf("попытка %d заблокирована раньше времени", i+1)
		}
		l.Fail("1.2.3.4|ivan")
	}

	ok, retry := l.Allow("1.2.3.4|ivan")
	if ok {
		t.Fatal("после 3 неудач лимитер обязан блокировать")
	}
	if retry <= 0 {
		t.Fatalf("retryAfter должен быть положительным, получен %v", retry)
	}

	// другой ключ (другой IP) не затронут
	if ok, _ := l.Allow("5.6.7.8|ivan"); !ok {
		t.Fatal("блокировка протекла на другой ключ")
	}
}

func TestLoginLimiter_ResetClearsFailures(t *testing.T) {
	l := auth.NewLoginLimiter(2, time.Minute)
	l.Fail("k")
	l.Fail("k")
	if ok, _ := l.Allow("k"); ok {
		t.Fatal("должен быть заблокирован")
	}
	l.Reset("k")
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("после Reset блокировка должна сняться")
	}
}

func TestLoginLimiter_BlockExpires(t *testing.T) {
	l := auth.NewLoginLimiter(1, 20*time.Millisecond)
	l.Fail("k")
	if ok, _ := l.Allow("k"); ok {
		t.Fatal("должен быть заблокирован")
	}
	time.Sleep(40 * time.Millisecond)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("блокировка должна истечь по окну")
	}
}

// Хендлер логина: после N неудач — 429 + Retry-After, даже с верным паролем;
// после успешного входа в новом окне счётчик сбрасывается.
func TestLoginSubmit_RateLimited(t *testing.T) {
	repo, ctx := newTestRepo(t)
	if _, err := repo.Create(ctx, "ivan", "secret123", "Иван", false); err != nil {
		t.Fatalf("Create: %v", err)
	}
	h := &auth.Handlers{Repo: repo, LoginLimit: auth.NewLoginLimiter(3, time.Minute)}

	post := func(password string) *httptest.ResponseRecorder {
		form := url.Values{"login": {"ivan"}, "password": {password}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "10.0.0.1:55555"
		rec := httptest.NewRecorder()
		h.LoginSubmit(rec, req)
		return rec
	}

	for i := 0; i < 3; i++ {
		if rec := post("wrong"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("попытка %d: ожидался 401, получен %d", i+1, rec.Code)
		}
	}

	rec := post("secret123") // верный пароль, но лимит исчерпан
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("после 3 неудач ожидался 429, получен %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("429 без заголовка Retry-After")
	}
}

func TestLoginJSON_RateLimited(t *testing.T) {
	repo, ctx := newTestRepo(t)
	if _, err := repo.Create(ctx, "ivan", "secret123", "Иван", false); err != nil {
		t.Fatalf("Create: %v", err)
	}
	h := &auth.Handlers{Repo: repo, LoginLimit: auth.NewLoginLimiter(2, time.Minute)}

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
		req.RemoteAddr = "10.0.0.2:1234"
		rec := httptest.NewRecorder()
		h.LoginJSON(rec, req)
		return rec
	}

	bad := `{"login":"ivan","password":"wrong"}`
	for i := 0; i < 2; i++ {
		if rec := post(bad); rec.Code != http.StatusUnauthorized {
			t.Fatalf("попытка %d: ожидался 401, получен %d", i+1, rec.Code)
		}
	}
	if rec := post(bad); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ожидался 429, получен %d", rec.Code)
	}
}

// Успешный вход сбрасывает счётчик неудач для ключа.
func TestLoginSubmit_SuccessResetsLimiter(t *testing.T) {
	repo, ctx := newTestRepo(t)
	if _, err := repo.Create(ctx, "ivan", "secret123", "Иван", false); err != nil {
		t.Fatalf("Create: %v", err)
	}
	limiter := auth.NewLoginLimiter(3, time.Minute)
	h := &auth.Handlers{Repo: repo, LoginLimit: limiter}

	post := func(password string) *httptest.ResponseRecorder {
		form := url.Values{"login": {"ivan"}, "password": {password}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "10.0.0.3:7777"
		rec := httptest.NewRecorder()
		h.LoginSubmit(rec, req)
		return rec
	}

	post("wrong")
	post("wrong")
	if rec := post("secret123"); rec.Code != http.StatusFound {
		t.Fatalf("верный пароль до блокировки: ожидался 302, получен %d", rec.Code)
	}
	// счётчик сброшен — две новые неудачи не блокируют
	post("wrong")
	if rec := post("wrong"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("после успешного входа счётчик должен был сброситься, получен %d", rec.Code)
	}
}
