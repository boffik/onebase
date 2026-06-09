package entityservice

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/storage"
)

func TestSave_PostingBlockedWhenMarked(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	doc := &metadata.Entity{
		Name:    "Расходник",
		Kind:    metadata.KindDocument,
		Posting: true,
		Fields:  []metadata.Field{{Name: "Номер", Type: metadata.FieldTypeString}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{doc}); err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if err := db.Upsert(ctx, doc.Name, id, map[string]any{"Номер": "Р-1"}, doc); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkForDeletion(ctx, doc.Name, id, true); err != nil {
		t.Fatal(err)
	}

	reg := runtime.NewRegistry()
	reg.Load(runtime.LoadOptions{Entities: []*metadata.Entity{doc}})
	svc := &Service{Store: db, Reg: reg, Interp: interpreter.New()}

	res, err := svc.Save(ctx, SaveRequest{
		Entity: doc, ID: id, IsNew: false,
		Fields: map[string]any{"Номер": "Р-1"}, Action: "post",
	})
	if err != nil {
		t.Fatalf("ожидалась бизнес-ошибка через DSLError, получили err=%v", err)
	}
	if res.DSLError == "" {
		t.Fatal("ожидался res.DSLError != \"\" (проведение помеченного запрещено)")
	}
	var posted bool
	db.QueryRow(ctx, "SELECT posted FROM расходник LIMIT 1").Scan(&posted)
	if posted {
		t.Error("документ не должен быть проведён")
	}
}
