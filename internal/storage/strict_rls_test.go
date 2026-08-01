package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

// Strict-RLS чокпоинт (план 79F): List для сущности с политикой, запрошенный без
// вычисленного строкового доступа, отклоняется fail-closed. По умолчанию (guard
// nil) режим выключен и поведение не меняется.
func TestStrictRLSGuard_ListChokepoint(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	guarded := &metadata.Entity{Name: "Заказы", Kind: metadata.KindDocument,
		Fields: []metadata.Field{{Name: "Номер", Type: metadata.FieldTypeString}}}
	open := &metadata.Entity{Name: "Валюты", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Код", Type: metadata.FieldTypeString}}}
	if err := db.Migrate(ctx, []*metadata.Entity{guarded, open}); err != nil {
		t.Fatal(err)
	}

	// По умолчанию (guard nil) чокпоинт выключен — список без флага проходит.
	if _, err := db.List(ctx, guarded.Name, guarded, ListParams{}); err != nil {
		t.Fatalf("guard nil: список должен проходить, получили %v", err)
	}

	// Строгий режим: политика есть только у «заказы».
	db.SetStrictRLSGuard(func(name string) bool { return name == "заказы" })

	// Guarded без вычисленного доступа → fail-closed.
	_, err = db.List(ctx, guarded.Name, guarded, ListParams{})
	if err == nil {
		t.Fatal("strict RLS: ожидалась ошибка для guarded-сущности без RowFilterEvaluated")
	}
	if !strings.Contains(err.Error(), "strict RLS") {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	// Guarded с вычисленным доступом → проходит (даже если фильтр nil — неограничивающая политика).
	if _, err := db.List(ctx, guarded.Name, guarded, ListParams{RowFilterEvaluated: true}); err != nil {
		t.Fatalf("guarded + RowFilterEvaluated: список должен проходить, получили %v", err)
	}

	// Non-guarded без флага → проходит (нет политики — нечего обходить).
	if _, err := db.List(ctx, open.Name, open, ListParams{}); err != nil {
		t.Fatalf("non-guarded: список должен проходить, получили %v", err)
	}

	// Выключение режима (nil) снова открывает.
	db.SetStrictRLSGuard(nil)
	if _, err := db.List(ctx, guarded.Name, guarded, ListParams{}); err != nil {
		t.Fatalf("после выключения: список должен проходить, получили %v", err)
	}
}
