// Package metrics реализует минимальный, без внешних зависимостей, сбор
// HTTP-метрик и их отдачу в текстовом формате Prometheus exposition.
//
// Зачем своё, а не github.com/prometheus/client_golang: go.mod проекта намеренно
// лёгкий, а нам нужно ровно три вещи — счётчик запросов, гистограмма латентности
// и несколько gauge для пула БД. Всё это умещается в пару сотен строк и не тянет
// десяток транзитивных зависимостей.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// DefaultBuckets — границы гистограммы латентности HTTP-запросов в секундах.
// Подобраны под web-нагрузку: от 5 мс до 10 с.
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type reqKey struct{ method, route, status string }
type routeKey struct{ method, route string }

type histogram struct {
	// counts[i] — число наблюдений в корзине i (значение <= buckets[i]).
	// Последний элемент (индекс len(buckets)) — корзина +Inf.
	counts []uint64
	sum    float64
	count  uint64
}

// Registry хранит HTTP-метрики процесса. Потокобезопасен.
type Registry struct {
	mu        sync.Mutex
	buckets   []float64
	requests  map[reqKey]uint64
	durations map[routeKey]*histogram
}

// New создаёт реестр с корзинами гистограммы по умолчанию.
func New() *Registry {
	return &Registry{
		buckets:   DefaultBuckets,
		requests:  make(map[reqKey]uint64),
		durations: make(map[routeKey]*histogram),
	}
}

func (reg *Registry) observe(method, route string, status int, d time.Duration) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	reg.requests[reqKey{method, route, strconv.Itoa(status)}]++

	rk := routeKey{method, route}
	h := reg.durations[rk]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(reg.buckets)+1)}
		reg.durations[rk] = h
	}
	sec := d.Seconds()
	h.sum += sec
	h.count++
	idx := sort.SearchFloat64s(reg.buckets, sec) // первый bucket >= sec
	h.counts[idx]++
}

// statusRecorder перехватывает HTTP-код ответа для метрик. Своя обёртка, чтобы
// не зависеть от деталей chi middleware.WrapResponseWriter.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sr *statusRecorder) WriteHeader(code int) {
	if !sr.wroteHeader {
		sr.status = code
		sr.wroteHeader = true
	}
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if !sr.wroteHeader {
		sr.status = http.StatusOK
		sr.wroteHeader = true
	}
	return sr.ResponseWriter.Write(b)
}

// Middleware записывает по каждому запросу счётчик и латентность. Метку route
// берём из chi RoutePattern (доступен после маршрутизации) — это шаблон вида
// «/documents/{entity}/{id}/post», что держит кардинальность меток низкой
// (а не плодит серию на каждый id). Незаматченные пути группируются как "other".
func (reg *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "other"
		}
		reg.observe(r.Method, route, sr.status, time.Since(start))
	})
}

// WritePrometheus печатает накопленные метрики в формате Prometheus exposition.
func (reg *Registry) WritePrometheus(w io.Writer) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	// ── counter: onebase_http_requests_total ──────────────────────────────
	fmt.Fprintln(w, "# HELP onebase_http_requests_total Общее число обработанных HTTP-запросов.")
	fmt.Fprintln(w, "# TYPE onebase_http_requests_total counter")
	reqKeys := make([]reqKey, 0, len(reg.requests))
	for k := range reg.requests {
		reqKeys = append(reqKeys, k)
	}
	sort.Slice(reqKeys, func(i, j int) bool {
		a, b := reqKeys[i], reqKeys[j]
		if a.route != b.route {
			return a.route < b.route
		}
		if a.method != b.method {
			return a.method < b.method
		}
		return a.status < b.status
	})
	for _, k := range reqKeys {
		fmt.Fprintf(w, "onebase_http_requests_total{method=%q,route=%q,status=%q} %d\n",
			k.method, k.route, k.status, reg.requests[k])
	}

	// ── histogram: onebase_http_request_duration_seconds ──────────────────
	fmt.Fprintln(w, "# HELP onebase_http_request_duration_seconds Латентность HTTP-запросов в секундах.")
	fmt.Fprintln(w, "# TYPE onebase_http_request_duration_seconds histogram")
	durKeys := make([]routeKey, 0, len(reg.durations))
	for k := range reg.durations {
		durKeys = append(durKeys, k)
	}
	sort.Slice(durKeys, func(i, j int) bool {
		a, b := durKeys[i], durKeys[j]
		if a.route != b.route {
			return a.route < b.route
		}
		return a.method < b.method
	})
	for _, k := range durKeys {
		h := reg.durations[k]
		var cum uint64
		for i, ub := range reg.buckets {
			cum += h.counts[i]
			fmt.Fprintf(w, "onebase_http_request_duration_seconds_bucket{method=%q,route=%q,le=%q} %d\n",
				k.method, k.route, strconv.FormatFloat(ub, 'g', -1, 64), cum)
		}
		cum += h.counts[len(reg.buckets)] // +Inf
		fmt.Fprintf(w, "onebase_http_request_duration_seconds_bucket{method=%q,route=%q,le=\"+Inf\"} %d\n",
			k.method, k.route, cum)
		fmt.Fprintf(w, "onebase_http_request_duration_seconds_sum{method=%q,route=%q} %s\n",
			k.method, k.route, strconv.FormatFloat(h.sum, 'g', -1, 64))
		fmt.Fprintf(w, "onebase_http_request_duration_seconds_count{method=%q,route=%q} %d\n",
			k.method, k.route, h.count)
	}
}
