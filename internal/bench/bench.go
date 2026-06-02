// Package bench реализует синтетический нагрузочный тест onebase в стиле теста
// Гилёва: фиксированная эталонная конфигурация + канонический сценарий
// «создать документ → провести» в N потоков, на выходе — один сравнимый балл
// (документов/сек), перцентили латентности и APDEX.
//
// Сценарий ходит во внутренние пакеты (entityservice → storage), а не по HTTP:
// это меряет ядро платформы (интерпретатор OnPost + транзакция движений) без
// накладных сети, что и есть аналог «проведения документов» из TPC-1C-GILV.
// HTTP-слой нагружается отдельно через k6 (см. loadtest/).
package bench

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/storage"
)

// onPostSrc — DSL-хук проведения эталонного документа: одно движение по
// регистру накопления из реквизитов шапки. Простой, но репрезентативный путь:
// интерпретатор + сбор движений + запись в регистр в транзакции.
const onPostSrc = `Процедура OnPost()
  Дв = Движения.ОстаткиТоваров.Добавить();
  Дв.Номенклатура = this.Номенклатура;
  Дв.Количество = this.Количество;
  Дв.Сумма = this.Количество * this.Цена;
КонецПроцедуры`

// Options управляет прогоном Harness.Run.
type Options struct {
	Threads  int           // число параллельных воркеров (>=1)
	Duration time.Duration // длительность измеряемого окна (если Count==0)
	Count    int           // если >0 — гнать ровно столько операций (вместо Duration)
	Warmup   time.Duration // прогрев перед измерением (результаты отбрасываются)
	ApdexT   time.Duration // порог T для APDEX (satisfied при латентности <= T)
}

// Result — агрегированный итог прогона.
type Result struct {
	Threads    int           `json:"threads"`
	Ops        int           `json:"ops"`
	Errors     int           `json:"errors"`
	Elapsed    time.Duration `json:"elapsed_ns"`
	Throughput float64       `json:"docs_per_sec"`
	Min        time.Duration `json:"min_ns"`
	Mean       time.Duration `json:"mean_ns"`
	P50        time.Duration `json:"p50_ns"`
	P95        time.Duration `json:"p95_ns"`
	P99        time.Duration `json:"p99_ns"`
	Max        time.Duration `json:"max_ns"`
	Apdex      float64       `json:"apdex"`
	ApdexT     time.Duration `json:"apdex_t_ns"`
}

// Harness держит готовый к работе entityservice над эталонной базой.
type Harness struct {
	svc *entityserviceFacade
	doc *metadata.Entity
}

// referenceBase описывает эталонную конфигурацию в коде (а не из YAML) — это
// гарантирует воспроизводимость балла между версиями и машинами.
func referenceBase() (*metadata.Entity, *metadata.Register, *ast.Program, error) {
	doc := &metadata.Entity{
		Name:    "Поступление",
		Kind:    metadata.KindDocument,
		Posting: true,
		Fields: []metadata.Field{
			{Name: "Дата", Type: metadata.FieldTypeDate},
			{Name: "Номенклатура", Type: metadata.FieldTypeString},
			{Name: "Количество", Type: metadata.FieldTypeNumber},
			{Name: "Цена", Type: metadata.FieldTypeNumber},
		},
	}
	reg := &metadata.Register{
		Name:       "ОстаткиТоваров",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: metadata.FieldTypeString}},
		Resources: []metadata.Field{
			{Name: "Количество", Type: metadata.FieldTypeNumber},
			{Name: "Сумма", Type: metadata.FieldTypeNumber},
		},
	}
	prog, err := parser.New(lexer.New(onPostSrc, "bench.os")).ParseProgram()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bench: parse OnPost: %w", err)
	}
	return doc, reg, prog, nil
}

// NewHarness применяет схему эталонной базы к db и собирает entityservice.
// db может быть как PostgreSQL (storage.Connect), так и SQLite
// (storage.ConnectSQLite) — для абсолютной оценки используйте PostgreSQL.
func NewHarness(ctx context.Context, db *storage.DB) (*Harness, error) {
	doc, reg, prog, err := referenceBase()
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, []*metadata.Entity{doc}); err != nil {
		return nil, fmt.Errorf("bench: migrate doc: %w", err)
	}
	if err := db.MigrateRegisters(ctx, []*metadata.Register{reg}); err != nil {
		return nil, fmt.Errorf("bench: migrate register: %w", err)
	}

	registry := runtime.NewRegistry()
	registry.Load(runtime.LoadOptions{
		Entities:  []*metadata.Entity{doc},
		Registers: []*metadata.Register{reg},
		Programs:  map[string]*ast.Program{doc.Name: prog},
	})
	interp := interpreter.New()
	interp.LookupProc = registry.GetModuleProc

	svc := newEntityserviceFacade(db, registry, interp)
	return &Harness{svc: svc, doc: doc}, nil
}

// postOne создаёт и проводит один документ, возвращая латентность операции.
func (h *Harness) postOne(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	err := h.svc.postDocument(ctx, h.doc, uuid.New(), map[string]any{
		"Дата":         start,
		"Номенклатура": "Гвоздь",
		"Количество":   float64(10),
		"Цена":         float64(5),
	})
	return time.Since(start), err
}

// Run выполняет прогрев и измеряемое окно, возвращая агрегат.
func (h *Harness) Run(ctx context.Context, opts Options) (Result, error) {
	threads := opts.Threads
	if threads < 1 {
		threads = 1
	}
	if opts.Warmup > 0 {
		h.runPhase(ctx, threads, opts.Warmup, 0)
	}
	lat, errs, elapsed := h.runPhase(ctx, threads, opts.Duration, opts.Count)
	return summarize(lat, errs, elapsed, threads, opts.ApdexT), nil
}

// runPhase гоняет threads воркеров либо в течение dur (если count==0), либо
// ровно count операций суммарно. Возвращает латентности успешных операций,
// число ошибок и фактически прошедшее время.
func (h *Harness) runPhase(ctx context.Context, threads int, dur time.Duration, count int) ([]time.Duration, int, time.Duration) {
	useCount := count > 0

	phaseCtx := ctx
	var cancel context.CancelFunc
	if !useCount {
		phaseCtx, cancel = context.WithTimeout(ctx, dur)
		defer cancel()
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		all       []time.Duration
		errCount  int64
		remaining = int64(count)
	)

	start := time.Now()
	for w := 0; w < threads; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]time.Duration, 0, 1024)
			for {
				if useCount {
					if atomic.AddInt64(&remaining, -1) < 0 {
						break
					}
				} else if phaseCtx.Err() != nil {
					break
				}
				// Саму операцию выполняем на исходном ctx, а не на phaseCtx:
				// дедлайн не должен оборвать незавершённую транзакцию записи.
				d, err := h.postOne(ctx)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					continue
				}
				local = append(local, d)
			}
			mu.Lock()
			all = append(all, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return all, int(errCount), time.Since(start)
}

func summarize(lat []time.Duration, errs int, elapsed time.Duration, threads int, apdexT time.Duration) Result {
	res := Result{
		Threads: threads,
		Ops:     len(lat),
		Errors:  errs,
		Elapsed: elapsed,
		ApdexT:  apdexT,
	}
	if elapsed > 0 {
		res.Throughput = float64(len(lat)) / elapsed.Seconds()
	}
	if len(lat) == 0 {
		return res
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	var sum time.Duration
	for _, d := range lat {
		sum += d
	}
	res.Min = lat[0]
	res.Max = lat[len(lat)-1]
	res.Mean = sum / time.Duration(len(lat))
	res.P50 = percentile(lat, 0.50)
	res.P95 = percentile(lat, 0.95)
	res.P99 = percentile(lat, 0.99)
	res.Apdex = apdex(lat, apdexT)
	return res
}

// percentile возвращает p-й перцентиль из ОТСОРТИРОВАННОГО по возрастанию
// среза (метод «nearest rank»). p в долях: 0.95 == p95.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(p*float64(len(sorted)) + 0.999999) // ceil
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

// apdex считает индекс по стандартной формуле: (satisfied + tolerated/2) / total,
// где satisfied — латентность <= T, tolerated — (T, 4T], frustrated — > 4T.
func apdex(lat []time.Duration, t time.Duration) float64 {
	if t <= 0 || len(lat) == 0 {
		return 0
	}
	var satisfied, tolerated int
	tol := 4 * t
	for _, d := range lat {
		switch {
		case d <= t:
			satisfied++
		case d <= tol:
			tolerated++
		}
	}
	return (float64(satisfied) + float64(tolerated)/2) / float64(len(lat))
}
