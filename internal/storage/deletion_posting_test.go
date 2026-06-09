package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

func TestIsMarkedForDeletionAndPostingGuard(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
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

	// Не помечен → false.
	if marked, err := db.IsMarkedForDeletion(ctx, doc.Name, id); err != nil || marked {
		t.Fatalf("ожидался (false,nil), получили (%v,%v)", marked, err)
	}
	// Несуществующий id → false без ошибки.
	if marked, err := db.IsMarkedForDeletion(ctx, doc.Name, uuid.New()); err != nil || marked {
		t.Fatalf("несуществующий: ожидался (false,nil), получили (%v,%v)", marked, err)
	}
	// До пометки SetPosted(true) работает.
	if err := db.SetPosted(ctx, doc.Name, id, true); err != nil {
		t.Fatalf("SetPosted(true) до пометки: %v", err)
	}
	// Снять проведение, пометить на удаление.
	if err := db.SetPosted(ctx, doc.Name, id, false); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkForDeletion(ctx, doc.Name, id, true); err != nil {
		t.Fatal(err)
	}
	if marked, _ := db.IsMarkedForDeletion(ctx, doc.Name, id); !marked {
		t.Fatal("после MarkForDeletion ожидался marked=true")
	}
	// SetPosted(true) на помеченном → ErrPostingDeletionMarked.
	if err := db.SetPosted(ctx, doc.Name, id, true); !errors.Is(err, ErrPostingDeletionMarked) {
		t.Fatalf("ожидался ErrPostingDeletionMarked, получили %v", err)
	}
	// SetPosted(false) (отмена проведения) на помеченном всё ещё работает.
	if err := db.SetPosted(ctx, doc.Name, id, false); err != nil {
		t.Fatalf("SetPosted(false) на помеченном должен работать: %v", err)
	}
}
