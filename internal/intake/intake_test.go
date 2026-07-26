package intake_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/intake"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/storage"
)

// setup поднимает SQLite с таблицами приёмки и «бизнес-таблицей» zayavki, на
// которой проверяется транзакционность обработчика.
func setup(t *testing.T) (*intake.Engine, *storage.DB, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "intake.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.EnsureIntakeSchema(ctx); err != nil {
		t.Fatalf("EnsureIntakeSchema: %v", err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE zayavki (id TEXT PRIMARY KEY, phone TEXT)`); err != nil {
		t.Fatalf("create zayavki: %v", err)
	}
	return intake.New(db), db, ctx
}

func sampleIntake() *metadata.Intake {
	in := &metadata.Intake{
		Name: "SiteLead", Transport: "http", Endpoint: "/hs/site/lead", Handler: "SiteLead",
		Idempotency: metadata.IntakeIdempotency{Key: "event_id", Scope: []string{"source", "aggregate"}},
	}
	in.Normalize()
	return in
}

func env(t *testing.T, eventID string, payload map[string]any) intake.Envelope {
	t.Helper()
	top := map[string]any{
		"event_id": eventID, "source": "site", "aggregate": "Заявка",
		"correlation_id": "corr-" + eventID, "payload": payload,
	}
	raw, err := json.Marshal(top)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	e, err := intake.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	return e
}

// creatingHandler вставляет заявку в той же транзакции приёмки и считает вызовы.
func creatingHandler(db *storage.DB, calls *int64) intake.HandlerFunc {
	return func(ctx context.Context, e intake.Envelope) (intake.HandlerResult, error) {
		atomic.AddInt64(calls, 1)
		id := uuid.NewString()
		phone, _ := e.Payload["phone"].(string)
		if _, err := db.Exec(ctx, `INSERT INTO zayavki (id, phone) VALUES (?, ?)`, id, phone); err != nil {
			return intake.HandlerResult{}, err
		}
		return intake.HandlerResult{Ref: id, BusinessResult: map[string]any{"phone": phone}}, nil
	}
}

// failingHandler создаёт заявку, но затем возвращает ошибку — проверка отката.
func failingHandler(db *storage.DB) intake.HandlerFunc {
	return func(ctx context.Context, e intake.Envelope) (intake.HandlerResult, error) {
		id := uuid.NewString()
		if _, err := db.Exec(ctx, `INSERT INTO zayavki (id, phone) VALUES (?, ?)`, id, "999"); err != nil {
			return intake.HandlerResult{}, err
		}
		return intake.HandlerResult{}, errors.New("обработчик упал")
	}
}

func count(t *testing.T, db *storage.DB, ctx context.Context, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestIngest_HappyPath(t *testing.T) {
	eng, db, ctx := setup(t)
	var calls int64
	res, err := eng.Ingest(ctx, sampleIntake(), creatingHandler(db, &calls), env(t, "E1", map[string]any{"phone": "111"}))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if res.Status != intake.StatusAccepted {
		t.Fatalf("статус=%q, ожидалось Принято", res.Status)
	}
	if res.ResultRef == "" {
		t.Fatalf("не заполнена ссылка результата")
	}
	if n := count(t, db, ctx, "zayavki"); n != 1 {
		t.Fatalf("заявок=%d, ожидалась 1", n)
	}
	row, _, _ := db.GetIntakeLog(ctx, "SiteLead", "site|Заявка", "E1")
	if row.Status != storage.IntakeStatusProcessed {
		t.Fatalf("строка идемпотентности не processed: %q", row.Status)
	}
}

func TestIngest_Duplicate(t *testing.T) {
	eng, db, ctx := setup(t)
	in := sampleIntake()
	var calls int64
	h := creatingHandler(db, &calls)
	payload := map[string]any{"phone": "111"}

	first, _ := eng.Ingest(ctx, in, h, env(t, "E1", payload))
	second, err := eng.Ingest(ctx, in, h, env(t, "E1", payload))
	if err != nil {
		t.Fatalf("Ingest дубль: %v", err)
	}
	if second.Status != intake.StatusDuplicate {
		t.Fatalf("статус=%q, ожидалось Дубль", second.Status)
	}
	if second.ResultRef != first.ResultRef {
		t.Fatalf("дубль вернул другую ссылку: %q != %q", second.ResultRef, first.ResultRef)
	}
	if atomic.LoadInt64(&calls) != 1 {
		t.Fatalf("обработчик вызван %d раз, ожидался 1 (без повторной обработки)", calls)
	}
	if n := count(t, db, ctx, "zayavki"); n != 1 {
		t.Fatalf("заявок=%d, ожидалась 1", n)
	}
}

func TestIngest_Mismatch(t *testing.T) {
	eng, db, ctx := setup(t)
	in := sampleIntake()
	var calls int64
	h := creatingHandler(db, &calls)

	eng.Ingest(ctx, in, h, env(t, "E1", map[string]any{"phone": "111"}))
	// Тот же event_id, другое тело → несовпадение → карантин.
	res, err := eng.Ingest(ctx, in, h, env(t, "E1", map[string]any{"phone": "222"}))
	if err != nil {
		t.Fatalf("Ingest mismatch: %v", err)
	}
	if res.Status != intake.StatusQuarantined || res.Reason != metadata.DLQSchemaMismatch {
		t.Fatalf("ожидался Карантин/schema_mismatch, получено %q/%q", res.Status, res.Reason)
	}
	if n := count(t, db, ctx, "zayavki"); n != 1 {
		t.Fatalf("заявок=%d, ожидалась 1 (вторая — карантин, без создания)", n)
	}
	open, _ := db.ListIntakeDLQ(ctx, "SiteLead", true, 0)
	if len(open) != 1 || open[0].Reason != metadata.DLQSchemaMismatch {
		t.Fatalf("ожидалась 1 запись карантина schema_mismatch, получено %d", len(open))
	}
}

// TestIngest_HandlerError_Rollback — сбой обработчика откатывает бизнес-объект
// целиком (транзакционная связка) и отправляет событие в карантин. Никаких
// полу-записей.
func TestIngest_HandlerError_Rollback(t *testing.T) {
	eng, db, ctx := setup(t)
	in := sampleIntake()

	res, err := eng.Ingest(ctx, in, failingHandler(db), env(t, "E1", map[string]any{"phone": "111"}))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if res.Status != intake.StatusQuarantined || res.Reason != metadata.DLQHandlerError {
		t.Fatalf("ожидался Карантин/handler_error, получено %q/%q", res.Status, res.Reason)
	}
	if n := count(t, db, ctx, "zayavki"); n != 0 {
		t.Fatalf("заявок=%d, ожидалось 0 (откат)", n)
	}
	row, _, _ := db.GetIntakeLog(ctx, "SiteLead", "site|Заявка", "E1")
	if row.Status != storage.IntakeStatusQuarantined {
		t.Fatalf("строка идемпотентности должна быть quarantined, получено %q", row.Status)
	}
	if open, _ := db.ListIntakeDLQ(ctx, "SiteLead", true, 0); len(open) != 1 {
		t.Fatalf("ожидалась 1 запись карантина, получено %d", len(open))
	}
}

// TestReplay восстанавливает событие из карантина: после починки обработчик
// проходит, заявка создаётся ровно одна, запись карантина закрывается.
func TestReplay(t *testing.T) {
	eng, db, ctx := setup(t)
	in := sampleIntake()

	// Сначала загоняем в карантин сбойным обработчиком.
	eng.Ingest(ctx, in, failingHandler(db), env(t, "E1", map[string]any{"phone": "111"}))

	var calls int64
	res, err := eng.Replay(ctx, in, creatingHandler(db, &calls), "E1")
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if res.Status != intake.StatusAccepted || res.ResultRef == "" {
		t.Fatalf("ожидалось Принято со ссылкой, получено %q/%q", res.Status, res.ResultRef)
	}
	if n := count(t, db, ctx, "zayavki"); n != 1 {
		t.Fatalf("заявок=%d, ожидалась 1 (сбойный проход ничего не закоммитил)", n)
	}
	if open, _ := db.ListIntakeDLQ(ctx, "SiteLead", true, 0); len(open) != 0 {
		t.Fatalf("после replay открытых записей быть не должно, есть %d", len(open))
	}
	row, _, _ := db.GetIntakeLog(ctx, "SiteLead", "site|Заявка", "E1")
	if row.Status != storage.IntakeStatusProcessed {
		t.Fatalf("после replay строка должна быть processed, получено %q", row.Status)
	}
}

func TestIngest_MissingKey(t *testing.T) {
	eng, db, ctx := setup(t)
	var calls int64
	// Конверт без event_id.
	raw, _ := json.Marshal(map[string]any{"source": "site", "payload": map[string]any{"phone": "1"}})
	e, _ := intake.ParseEnvelope(raw)
	res, err := eng.Ingest(ctx, sampleIntake(), creatingHandler(db, &calls), e)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if res.Status != intake.StatusRejected {
		t.Fatalf("ожидалось Отклонено, получено %q", res.Status)
	}
	if atomic.LoadInt64(&calls) != 0 {
		t.Fatalf("обработчик не должен вызываться без ключа")
	}
}

// TestIngest_Concurrency — конкурентная приёмка одного event_id: ровно одна
// заявка, ровно одно Принято, обработчик вызван один раз. Доказывает, что
// атомарный insert-if-new гейтит обработку.
func TestIngest_Concurrency(t *testing.T) {
	eng, db, ctx := setup(t)
	in := sampleIntake()
	var calls int64
	h := creatingHandler(db, &calls)
	payload := map[string]any{"phone": "111"}

	const goroutines = 30
	var start, done sync.WaitGroup
	start.Add(1)
	var accepted int64
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			res, err := eng.Ingest(ctx, in, h, env(t, "E1", payload))
			if err != nil {
				errs <- err
				return
			}
			if res.Status == intake.StatusAccepted {
				atomic.AddInt64(&accepted, 1)
			}
		}()
	}
	start.Done()
	done.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("Ingest concurrency error: %v", e)
	}
	if accepted != 1 {
		t.Fatalf("Принято=%d, ожидалось ровно 1", accepted)
	}
	if c := atomic.LoadInt64(&calls); c != 1 {
		t.Fatalf("обработчик вызван %d раз, ожидался 1", c)
	}
	if n := count(t, db, ctx, "zayavki"); n != 1 {
		t.Fatalf("заявок=%d, ожидалась 1", n)
	}
}
