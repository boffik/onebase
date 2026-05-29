package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/shopspring/decimal"
)

// Число с точностью округляется при записи — единое поведение PG и SQLite
// (SQLite хранит TEXT и сам не округляет).
func TestNumberSpec_RoundOnWriteSQLite(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	entity := &metadata.Entity{
		Name: "Товар",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Цена", Type: metadata.FieldTypeNumber, Length: 10, Scale: 2},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{entity}); err != nil {
		t.Fatal(err)
	}

	id := uuid.New()
	if err := db.Upsert(ctx, "Товар", id, map[string]any{
		"Цена": decimal.RequireFromString("10.999"),
	}, entity); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	row, err := db.GetByID(ctx, "Товар", id, entity)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	got := row["Цена"].(decimal.Decimal)
	if got.String() != "11" {
		t.Errorf("округление до scale=2: 10.999 → %q, ожидалось 11", got.String())
	}
}

// Переполнение по числу целых разрядов даёт понятную ошибку, а не молчаливую
// запись (SQLite) / numeric overflow (PG).
func TestNumberSpec_OverflowError(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	entity := &metadata.Entity{
		Name: "Товар",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Цена", Type: metadata.FieldTypeNumber, Length: 5, Scale: 2}, // макс 3 целых разряда
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{entity}); err != nil {
		t.Fatal(err)
	}

	err = db.Upsert(ctx, "Товар", uuid.New(), map[string]any{
		"Цена": decimal.RequireFromString("1234.5"), // 4 целых разряда > 3
	}, entity)
	if err == nil {
		t.Fatal("ожидалась ошибка переполнения разрядности, получено nil")
	}
	if !strings.Contains(err.Error(), "разрядность") {
		t.Errorf("ошибка не про разрядность: %v", err)
	}
}

func TestFieldType_NumberSpecDDL(t *testing.T) {
	f := metadata.Field{Name: "Цена", Type: metadata.FieldTypeNumber, Length: 10, Scale: 2}
	if got := fieldType(PgDialect{}, f); got != "NUMERIC(10,2)" {
		t.Errorf("PG fieldType = %q, want NUMERIC(10,2)", got)
	}
	if got := fieldType(SQLiteDialect{}, f); got != "TEXT" {
		t.Errorf("SQLite fieldType = %q, want TEXT", got)
	}
	// Без разрядности — NUMERIC без параметров.
	plain := metadata.Field{Name: "Кол", Type: metadata.FieldTypeNumber}
	if got := fieldType(PgDialect{}, plain); got != "NUMERIC" {
		t.Errorf("PG plain number = %q, want NUMERIC", got)
	}
}
