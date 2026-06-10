package api

import "testing"

// План 53, этап 4: secure-by-default bind — сервер слушает 127.0.0.1, если
// явно не попросили наружу (--host 0.0.0.0).

func TestListenAddr(t *testing.T) {
	cases := []struct {
		host string
		port int
		want string
	}{
		{"", 8080, "127.0.0.1:8080"}, // пустой host = loopback по умолчанию
		{"127.0.0.1", 9000, "127.0.0.1:9000"},
		{"0.0.0.0", 80, "0.0.0.0:80"},
		{"::1", 8080, "[::1]:8080"},
	}
	for _, c := range cases {
		if got := listenAddr(c.host, c.port); got != c.want {
			t.Errorf("listenAddr(%q, %d) = %q, want %q", c.host, c.port, got, c.want)
		}
	}
}

func TestIsLoopbackHost(t *testing.T) {
	loopback := []string{"", "127.0.0.1", "localhost", "LOCALHOST", "::1", "127.5.5.5"}
	for _, h := range loopback {
		if !IsLoopbackHost(h) {
			t.Errorf("IsLoopbackHost(%q) = false, want true", h)
		}
	}
	public := []string{"0.0.0.0", "192.168.1.5", "10.0.0.1", "example.com", "::"}
	for _, h := range public {
		if IsLoopbackHost(h) {
			t.Errorf("IsLoopbackHost(%q) = true, want false", h)
		}
	}
}
