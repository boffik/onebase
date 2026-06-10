package api

import "testing"

// Лог HTTP-запросов не должен содержать значения сессионных токенов и
// одноразовых кодов (план 53, этап 1: анализ §2.2 — токен утекал в stdout
// через middleware.Logger + ?_tk=).
func TestRedactURI(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/ui?_tk=secret123", "/ui?_tk=***"},
		{"/auth/bootstrap?code=abc123&return=/ui", "/auth/bootstrap?code=***&return=/ui"},
		{"/x?token=tok&y=2", "/x?token=***&y=2"},
		{"/x?api_key=k1&apikey=k2", "/x?api_key=***&apikey=***"},
		{"/x?password=p", "/x?password=***"},
		{"/x?TOKEN=tok", "/x?TOKEN=***"}, // регистронезависимо
		{"/plain", "/plain"},
		{"/q?a=1&b=2", "/q?a=1&b=2"}, // обычные параметры не трогаем
		{"/q?", "/q?"},
	}
	for _, c := range cases {
		if got := redactURI(c.in); got != c.want {
			t.Errorf("redactURI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
