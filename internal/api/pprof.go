package api

import (
	"crypto/subtle"
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
)

// mountPprof registers the standard net/http/pprof handlers under /debug/pprof,
// gated by the same internal token as the rest of the debug surface
// (ONEBASE_DEBUG_TOKEN). api.New mounts this only when a token is configured,
// so a plain `onebase run` exposes no profiling surface at all.
//
// The token may be supplied via the X-OneBase-Debug-Token header (как у
// остального debug API) ИЛИ через query-параметр ?token=… . Второе нужно для
// `go tool pprof http://host/debug/pprof/profile?token=…&seconds=30`, который
// не умеет ставить произвольные заголовки.
func mountPprof(r chi.Router, token string) {
	r.Route("/debug/pprof", func(r chi.Router) {
		r.Use(pprofTokenMiddleware(token))
		r.HandleFunc("/", pprof.Index)
		r.HandleFunc("/cmdline", pprof.Cmdline)
		r.HandleFunc("/profile", pprof.Profile)
		r.HandleFunc("/symbol", pprof.Symbol)
		r.HandleFunc("/trace", pprof.Trace)
		// Именованные профили (heap, goroutine, allocs, block, mutex,
		// threadcreate) обслуживает pprof.Index — он определяет имя по
		// r.URL.Path, который остаётся полным («/debug/pprof/heap»).
		r.HandleFunc("/{name}", pprof.Index)
	})
}

// pprofTokenMiddleware гейтит pprof тем же токеном, что и остальной debug API,
// но дополнительно принимает токен из query-параметра ?token= (см. mountPprof).
func pprofTokenMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("X-OneBase-Debug-Token")
			if got == "" {
				got = r.URL.Query().Get("token")
			}
			if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
