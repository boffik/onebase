package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/metrics"
	"github.com/ivantit66/onebase/internal/storage"
)

// mountMetrics вешает /metrics, отдающий HTTP-метрики (reg) и gauges пула
// соединений (store). Эндпоинт закрыт тем же токеном, что pprof и остальной
// debug API: токен в заголовке X-OneBase-Debug-Token или в query ?token=…
// (последнее удобно для prometheus scrape-конфига через params).
func mountMetrics(r chi.Router, token string, reg *metrics.Registry, store *storage.DB) {
	r.With(pprofTokenMiddleware(token)).Get("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		reg.WritePrometheus(w)
		writePoolStats(w, store)
	})
}

// writePoolStats дописывает gauges пула соединений PostgreSQL. Для SQLite пул
// отсутствует — метрики просто опускаются.
func writePoolStats(w http.ResponseWriter, store *storage.DB) {
	st := store.PoolStats()
	if st == nil {
		return
	}
	g := func(name, help string, val int64) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, val)
	}
	g("onebase_db_pool_acquired_conns", "Занятые соединения пула.", int64(st.AcquiredConns))
	g("onebase_db_pool_constructing_conns", "Соединения в процессе установки.", int64(st.ConstructingConns))
	g("onebase_db_pool_idle_conns", "Свободные соединения пула.", int64(st.IdleConns))
	g("onebase_db_pool_total_conns", "Всего соединений в пуле.", int64(st.TotalConns))
	g("onebase_db_pool_max_conns", "Максимум соединений пула.", int64(st.MaxConns))

	c := func(name, help string, val int64) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, val)
	}
	c("onebase_db_pool_acquire_total", "Всего успешных Acquire.", st.AcquireCount)
	c("onebase_db_pool_empty_acquire_total", "Acquire, ждавшие свободного соединения.", st.EmptyAcquireCount)
	c("onebase_db_pool_canceled_acquire_total", "Acquire, отменённые контекстом.", st.CanceledAcquireCount)
	c("onebase_db_pool_new_conns_total", "Всего созданных соединений.", st.NewConnsCount)
}
