package storage_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
)

func newIntakeDB(t *testing.T) (*storage.DB, context.Context) {
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
	// Повторный вызов должен быть безобиден (идемпотентность DDL).
	if err := db.EnsureIntakeSchema(ctx); err != nil {
		t.Fatalf("EnsureIntakeSchema (повтор): %v", err)
	}
	return db, ctx
}

func sampleRow() storage.IntakeLogRow {
	return storage.IntakeLogRow{
		Intake: "SiteLead", Scope: "site|Заявка", Key: "E1",
		PayloadHash: "hash1", Status: storage.IntakeStatusReceived,
		CorrelationID: "corr-1", ReceivedAt: 1000, TTLExpiresAt: 0,
	}
}

func TestInsertIntakeLogIfNew_Duplicate(t *testing.T) {
	db, ctx := newIntakeDB(t)

	ins, err := db.InsertIntakeLogIfNew(ctx, sampleRow())
	if err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	if !ins {
		t.Fatalf("первая вставка должна быть новой")
	}

	// Тот же ключ, даже с другим телом — не вставляется (дубль ключа).
	row2 := sampleRow()
	row2.PayloadHash = "hash2"
	ins, err = db.InsertIntakeLogIfNew(ctx, row2)
	if err != nil {
		t.Fatalf("insert 2: %v", err)
	}
	if ins {
		t.Fatalf("повторная вставка того же ключа должна вернуть inserted=false")
	}

	got, found, err := db.GetIntakeLog(ctx, "SiteLead", "site|Заявка", "E1")
	if err != nil || !found {
		t.Fatalf("GetIntakeLog: err=%v found=%v", err, found)
	}
	if got.PayloadHash != "hash1" {
		t.Fatalf("строка перезаписана: hash=%q, ожидалось hash1", got.PayloadHash)
	}
}

func TestInsertIntakeLogIfNew_ReclaimsExpiredTTL(t *testing.T) {
	db, ctx := newIntakeDB(t)
	old := sampleRow()
	old.Status = storage.IntakeStatusProcessed
	old.ReceivedAt = 10
	old.TTLExpiresAt = 100
	if inserted, err := db.InsertIntakeLogIfNew(ctx, old); err != nil || !inserted {
		t.Fatalf("insert old: inserted=%v err=%v", inserted, err)
	}

	fresh := sampleRow()
	fresh.PayloadHash = "hash2"
	fresh.ReceivedAt = 100
	fresh.TTLExpiresAt = 200
	inserted, err := db.InsertIntakeLogIfNew(ctx, fresh)
	if err != nil || !inserted {
		t.Fatalf("reclaim expired: inserted=%v err=%v", inserted, err)
	}
	got, found, err := db.GetIntakeLog(ctx, fresh.Intake, fresh.Scope, fresh.Key)
	if err != nil || !found {
		t.Fatalf("get reclaimed: found=%v err=%v", found, err)
	}
	if got.PayloadHash != "hash2" || got.ReceivedAt != 100 || got.TTLExpiresAt != 200 {
		t.Fatalf("истёкшая строка не заменена: %+v", got)
	}

	again := fresh
	again.PayloadHash = "hash3"
	again.ReceivedAt = 150
	if inserted, err := db.InsertIntakeLogIfNew(ctx, again); err != nil || inserted {
		t.Fatalf("неистёкшая строка не должна заменяться: inserted=%v err=%v", inserted, err)
	}

	pending := sampleRow()
	pending.Key = "pending"
	pending.ReceivedAt = 10
	pending.TTLExpiresAt = 20
	if inserted, err := db.InsertIntakeLogIfNew(ctx, pending); err != nil || !inserted {
		t.Fatalf("insert pending: inserted=%v err=%v", inserted, err)
	}
	pendingRetry := pending
	pendingRetry.ReceivedAt = 100
	pendingRetry.TTLExpiresAt = 200
	if inserted, err := db.InsertIntakeLogIfNew(ctx, pendingRetry); err != nil || inserted {
		t.Fatalf("истёкшая received-строка не должна вытесняться: inserted=%v err=%v", inserted, err)
	}
}

// TestInsertIntakeLogIfNew_Concurrency — крайне важный тест: ядро гарантии
// идемпотентности. 50 горутин конкурентно вставляют ОДИН и тот же ключ; ровно
// одна должна получить inserted=true. Это доказывает атомарный insert-if-new
// (UNIQUE + ON CONFLICT DO NOTHING) без гонки TOCTOU.
func TestInsertIntakeLogIfNew_Concurrency(t *testing.T) {
	db, ctx := newIntakeDB(t)

	const goroutines = 50
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)

	wins := make(chan bool, goroutines)
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			ins, err := db.InsertIntakeLogIfNew(ctx, sampleRow())
			if err != nil {
				errs <- err
				return
			}
			wins <- ins
		}()
	}
	start.Done()
	done.Wait()
	close(wins)
	close(errs)

	for e := range errs {
		t.Fatalf("InsertIntakeLogIfNew error: %v", e)
	}
	got := 0
	for w := range wins {
		if w {
			got++
		}
	}
	if got != 1 {
		t.Fatalf("inserted=true у %d горутин, ожидалась ровно 1", got)
	}
}

func TestSetIntakeLogProcessed(t *testing.T) {
	db, ctx := newIntakeDB(t)
	if _, err := db.InsertIntakeLogIfNew(ctx, sampleRow()); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.SetIntakeLogProcessed(ctx, "SiteLead", "site|Заявка", "E1", "ref-42", `{"ok":true}`); err != nil {
		t.Fatalf("SetIntakeLogProcessed: %v", err)
	}
	got, _, err := db.GetIntakeLog(ctx, "SiteLead", "site|Заявка", "E1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != storage.IntakeStatusProcessed || got.ResultRef != "ref-42" || got.BusinessResult != `{"ok":true}` {
		t.Fatalf("processed не записан: %+v", got)
	}
}

func TestIntakeDLQ_Lifecycle(t *testing.T) {
	db, ctx := newIntakeDB(t)
	id, err := db.InsertIntakeDLQ(ctx, storage.IntakeDLQEntry{
		Intake: "SiteLead", Key: "E9", Scope: "site|Заявка",
		Payload: `{"event_id":"E9"}`, Reason: storage.IntakeStatusQuarantined,
		Error: "boom", CorrelationID: "corr-9", QuarantinedAt: 2000,
	})
	if err != nil || id == "" {
		t.Fatalf("InsertIntakeDLQ: id=%q err=%v", id, err)
	}

	open, err := db.ListIntakeDLQ(ctx, "SiteLead", true, 0)
	if err != nil {
		t.Fatalf("ListIntakeDLQ: %v", err)
	}
	if len(open) != 1 || open[0].Key != "E9" {
		t.Fatalf("ожидалась 1 открытая запись, получено %d", len(open))
	}

	byID, found, err := db.GetIntakeDLQ(ctx, "SiteLead", id)
	if err != nil || !found || byID.Key != "E9" {
		t.Fatalf("GetIntakeDLQ: found=%v entry=%+v err=%v", found, byID, err)
	}
	if byID.Payload != `{"event_id":"E9"}` {
		t.Fatalf("payload не сохранён: %q", byID.Payload)
	}
	claimed, err := db.MarkIntakeDLQReplayedIfOpen(ctx, id)
	if err != nil || !claimed {
		t.Fatalf("MarkIntakeDLQReplayedIfOpen: claimed=%v err=%v", claimed, err)
	}
	claimed, err = db.MarkIntakeDLQReplayedIfOpen(ctx, id)
	if err != nil || claimed {
		t.Fatalf("повторный claim должен проиграть: claimed=%v err=%v", claimed, err)
	}
	byID, found, err = db.GetIntakeDLQ(ctx, "SiteLead", id)
	if err != nil || !found || byID.ReplayState != storage.DLQStateReplayed {
		t.Fatalf("после replayed неверное состояние: found=%v entry=%+v err=%v", found, byID, err)
	}
}
