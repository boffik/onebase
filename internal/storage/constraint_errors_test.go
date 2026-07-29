package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// TestUpsert_ForeignKeyViolation: запись справочника с reference-полем, которое
// указывает на несуществующий UUID, должна вернуть ErrForeignKeyViolation
// (а не сырую ошибку драйвера), чтобы REST-слой смог отдать 422 вместо 500.
func TestUpsert_ForeignKeyViolation(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	org := &metadata.Entity{
		Name:   "Организация",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	client := &metadata.Entity{
		Name: "Контрагент",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Организация", Type: "reference:Организация", RefEntity: "Организация"},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{org, client}); err != nil {
		t.Fatal(err)
	}

	// UUID синтаксически валиден, но такой организации нет.
	err = db.Upsert(ctx, "Контрагент", uuid.New(), map[string]any{
		"Наименование": "ООО Ромашка",
		"Организация":  "00000000-0000-0000-0000-000000000000",
	}, client)
	if err == nil {
		t.Fatal("ожидали ошибку нарушения внешнего ключа, получили nil")
	}
	if !errors.Is(err, ErrForeignKeyViolation) {
		t.Fatalf("ошибка = %v, не оборачивает ErrForeignKeyViolation", err)
	}
}

// TestUpsert_NoFalsePositive: успешная запись с валидной ссылкой не должна
// классифицироваться как нарушение FK, а прочие ошибки — не подменяться.
func TestUpsert_ForeignKeyValidRef(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	org := &metadata.Entity{
		Name:   "Организация",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	client := &metadata.Entity{
		Name: "Контрагент",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Организация", Type: "reference:Организация", RefEntity: "Организация"},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{org, client}); err != nil {
		t.Fatal(err)
	}

	orgID := uuid.New()
	if err := db.Upsert(ctx, "Организация", orgID, map[string]any{"Наименование": "Головная"}, org); err != nil {
		t.Fatalf("создать организацию: %v", err)
	}
	if err := db.Upsert(ctx, "Контрагент", uuid.New(), map[string]any{
		"Наименование": "ООО Ромашка",
		"Организация":  orgID.String(),
	}, client); err != nil {
		t.Fatalf("запись с валидной ссылкой не должна падать: %v", err)
	}
}
