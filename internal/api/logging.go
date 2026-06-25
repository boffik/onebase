package api

// Логирование HTTP-запросов с редактированием чувствительных query-параметров.
// middleware.Logger пишет полный RequestURI в stdout — значения сессионных
// токенов и одноразовых кодов в логе недопустимы (план 53, анализ §2.2).

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	oblog "github.com/ivantit66/onebase/internal/logging"
)

// redactURI заменяет значения чувствительных параметров на ***, сохраняя порядок
// и кодировку остальных. Оставлено как локальная обёртка для старых тестов API.
func redactURI(uri string) string {
	return oblog.RedactURI(uri)
}

// requestLogger logs HTTP requests as structured slog records with redacted URI.
func requestLogger() func(http.Handler) http.Handler {
	logger := oblog.Component("http")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				logger.LogAttrs(r.Context(), slog.LevelInfo, "http request",
					slog.String("method", r.Method),
					slog.String("uri", redactURI(r.RequestURI)),
					slog.Int("status", ww.Status()),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Int64("duration_ms", time.Since(start).Milliseconds()),
					slog.String("remote_addr", r.RemoteAddr),
					slog.String("request_id", middleware.GetReqID(r.Context())),
				)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
