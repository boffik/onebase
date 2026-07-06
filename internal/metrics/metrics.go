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
type opKey struct{ kind, status string }
type limitedOpKey struct{ kind, reason string }
type funcMetric struct {
	name       string
	help       string
	metricType string
	value      func() float64
}

type histogram struct {
	// counts[i] — число наблюдений в корзине i (значение <= buckets[i]).
	// Последний элемент (индекс len(buckets)) — корзина +Inf.
	counts []uint64
	sum    float64
	count  uint64
}

// Registry хранит HTTP-метрики процесса. Потокобезопасен.
type Registry struct {
	mu                 sync.Mutex
	buckets            []float64
	requests           map[reqKey]uint64
	durations          map[routeKey]*histogram
	operations         map[opKey]uint64
	operationDurations map[string]*histogram
	activeOperations   map[string]int64
	slowOperations     map[string]uint64
	limitedOperations  map[limitedOpKey]uint64
	funcMetrics        []funcMetric
}

// New создаёт реестр с корзинами гистограммы по умолчанию.
func New() *Registry {
	return &Registry{
		buckets:            DefaultBuckets,
		requests:           make(map[reqKey]uint64),
		durations:          make(map[routeKey]*histogram),
		operations:         make(map[opKey]uint64),
		operationDurations: make(map[string]*histogram),
		activeOperations:   make(map[string]int64),
		slowOperations:     make(map[string]uint64),
		limitedOperations:  make(map[limitedOpKey]uint64),
	}
}

// RegisterGaugeFunc registers a gauge whose value is sampled on scrape.
// Callbacks must not mutate this Registry and should use low-latency reads.
func (reg *Registry) RegisterGaugeFunc(name, help string, value func() float64) {
	reg.registerFuncMetric(name, help, "gauge", value)
}

// RegisterCounterFunc registers a monotonically increasing counter sampled on
// scrape. The callback is responsible for returning a cumulative value.
func (reg *Registry) RegisterCounterFunc(name, help string, value func() float64) {
	reg.registerFuncMetric(name, help, "counter", value)
}

func (reg *Registry) registerFuncMetric(name, help, metricType string, value func() float64) {
	if reg == nil || name == "" || help == "" || value == nil {
		return
	}
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.funcMetrics = append(reg.funcMetrics, funcMetric{
		name:       name,
		help:       help,
		metricType: metricType,
		value:      value,
	})
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

// OperationStart increments active operation gauge for kind. kind must be a
// low-cardinality value such as "report.run" or "http_service.run".
func (reg *Registry) OperationStart(kind string) {
	if reg == nil || kind == "" {
		return
	}
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.activeOperations[kind]++
}

// OperationFinish records duration/status and decrements the active operation
// gauge. status must be low-cardinality: ok/error/timeout/limited/canceled.
func (reg *Registry) OperationFinish(kind, status string, d time.Duration, slow bool) {
	if reg == nil || kind == "" {
		return
	}
	if status == "" {
		status = "unknown"
	}
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if reg.activeOperations[kind] > 0 {
		reg.activeOperations[kind]--
	}
	reg.operations[opKey{kind, status}]++
	h := reg.operationDurations[kind]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(reg.buckets)+1)}
		reg.operationDurations[kind] = h
	}
	sec := d.Seconds()
	h.sum += sec
	h.count++
	idx := sort.SearchFloat64s(reg.buckets, sec)
	h.counts[idx]++
	if slow {
		reg.slowOperations[kind]++
	}
}

// OperationLimited records a backpressure/limit hit without starting work.
func (reg *Registry) OperationLimited(kind, reason string) {
	if reg == nil || kind == "" {
		return
	}
	if reason == "" {
		reason = "unknown"
	}
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.limitedOperations[limitedOpKey{kind, reason}]++
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
	funcMetrics := append([]funcMetric(nil), reg.funcMetrics...)

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

	// ── operation counters/gauges: reports/export/processors/http services ──
	fmt.Fprintln(w, "# HELP onebase_operation_total Общее число тяжёлых runtime-операций.")
	fmt.Fprintln(w, "# TYPE onebase_operation_total counter")
	opKeys := make([]opKey, 0, len(reg.operations))
	for k := range reg.operations {
		opKeys = append(opKeys, k)
	}
	sort.Slice(opKeys, func(i, j int) bool {
		a, b := opKeys[i], opKeys[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		return a.status < b.status
	})
	for _, k := range opKeys {
		fmt.Fprintf(w, "onebase_operation_total{kind=%q,status=%q} %d\n", k.kind, k.status, reg.operations[k])
	}

	fmt.Fprintln(w, "# HELP onebase_active_operations Активные тяжёлые runtime-операции.")
	fmt.Fprintln(w, "# TYPE onebase_active_operations gauge")
	activeKeys := make([]string, 0, len(reg.activeOperations))
	for k := range reg.activeOperations {
		activeKeys = append(activeKeys, k)
	}
	sort.Strings(activeKeys)
	for _, k := range activeKeys {
		fmt.Fprintf(w, "onebase_active_operations{kind=%q} %d\n", k, reg.activeOperations[k])
	}

	fmt.Fprintln(w, "# HELP onebase_operation_duration_seconds Длительность тяжёлых runtime-операций в секундах.")
	fmt.Fprintln(w, "# TYPE onebase_operation_duration_seconds histogram")
	opDurKeys := make([]string, 0, len(reg.operationDurations))
	for k := range reg.operationDurations {
		opDurKeys = append(opDurKeys, k)
	}
	sort.Strings(opDurKeys)
	for _, kind := range opDurKeys {
		h := reg.operationDurations[kind]
		var cum uint64
		for i, ub := range reg.buckets {
			cum += h.counts[i]
			fmt.Fprintf(w, "onebase_operation_duration_seconds_bucket{kind=%q,le=%q} %d\n",
				kind, strconv.FormatFloat(ub, 'g', -1, 64), cum)
		}
		cum += h.counts[len(reg.buckets)]
		fmt.Fprintf(w, "onebase_operation_duration_seconds_bucket{kind=%q,le=\"+Inf\"} %d\n", kind, cum)
		fmt.Fprintf(w, "onebase_operation_duration_seconds_sum{kind=%q} %s\n",
			kind, strconv.FormatFloat(h.sum, 'g', -1, 64))
		fmt.Fprintf(w, "onebase_operation_duration_seconds_count{kind=%q} %d\n", kind, h.count)
	}

	fmt.Fprintln(w, "# HELP onebase_slow_operation_total Тяжёлые операции дольше slow_operation_ms.")
	fmt.Fprintln(w, "# TYPE onebase_slow_operation_total counter")
	slowKeys := make([]string, 0, len(reg.slowOperations))
	for k := range reg.slowOperations {
		slowKeys = append(slowKeys, k)
	}
	sort.Strings(slowKeys)
	for _, k := range slowKeys {
		fmt.Fprintf(w, "onebase_slow_operation_total{kind=%q} %d\n", k, reg.slowOperations[k])
	}

	fmt.Fprintln(w, "# HELP onebase_limited_operation_total Операции, отклонённые лимитами/backpressure.")
	fmt.Fprintln(w, "# TYPE onebase_limited_operation_total counter")
	limKeys := make([]limitedOpKey, 0, len(reg.limitedOperations))
	for k := range reg.limitedOperations {
		limKeys = append(limKeys, k)
	}
	sort.Slice(limKeys, func(i, j int) bool {
		a, b := limKeys[i], limKeys[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		return a.reason < b.reason
	})
	for _, k := range limKeys {
		fmt.Fprintf(w, "onebase_limited_operation_total{kind=%q,reason=%q} %d\n",
			k.kind, k.reason, reg.limitedOperations[k])
	}
	reg.mu.Unlock()
	writeFuncMetrics(w, funcMetrics)
}

func writeFuncMetrics(w io.Writer, metrics []funcMetric) {
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].name < metrics[j].name })
	for _, m := range metrics {
		fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
		fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.metricType)
		fmt.Fprintf(w, "%s %s\n", m.name, strconv.FormatFloat(safeMetricValue(m.value), 'g', -1, 64))
	}
}

func safeMetricValue(value func() float64) (out float64) {
	defer func() {
		if recover() != nil {
			out = 0
		}
	}()
	return value()
}
