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

	entry, found, err := db.GetOpenIntakeDLQ(ctx, "SiteLead", "E9")
	if err != nil || !found {
		t.Fatalf("GetOpenIntakeDLQ: found=%v err=%v", found, err)
	}
	if entry.Payload != `{"event_id":"E9"}` {
		t.Fatalf("payload не сохранён: %q", entry.Payload)
	}

	if err := db.SetIntakeDLQState(ctx, id, storage.DLQStateReplayed); err != nil {
		t.Fatalf("SetIntakeDLQState: %v", err)
	}
	_, found, err = db.GetOpenIntakeDLQ(ctx, "SiteLead", "E9")
	if err != nil {
		t.Fatalf("GetOpenIntakeDLQ после replay: %v", err)
	}
	if found {
		t.Fatalf("после replayed запись не должна считаться открытой")
	}
}
