package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/shopspring/decimal"
)

// План 42: числовое поле должно проходить запись→чтение через SQLite (колонка
// TEXT) без потери точности и возвращаться как decimal.Decimal.
func TestDecimal_RoundTripSQLite(t *testing.T) {
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
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Цена", Type: metadata.FieldTypeNumber},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{entity}); err != nil {
		t.Fatal(err)
	}

	cases := []string{"0.105", "123456789.99", "0.1", "10", "0.01"}
	for _, want := range cases {
		id := uuid.New()
		price := decimal.RequireFromString(want)
		if err := db.Upsert(ctx, "Товар", id, map[string]any{
			"Наименование": "X",
			"Цена":         price,
		}, entity); err != nil {
			t.Fatalf("Upsert %s: %v", want, err)
		}
		row, err := db.GetByID(ctx, "Товар", id, entity)
		if err != nil {
			t.Fatalf("GetByID %s: %v", want, err)
		}
		got, ok := row["Цена"].(decimal.Decimal)
		if !ok {
			t.Fatalf("Цена %s: тип %T, ожидался decimal.Decimal", want, row["Цена"])
		}
		if got.String() != want {
			t.Errorf("round-trip %s → %q", want, got.String())
		}
	}
}
