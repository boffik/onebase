package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

// MatchCatalogByField на реальном SQLite: корректно различает 0 / 1 / несколько
// совпадений и отдаёт точное количество дублей. Это движок safe-match API
// (Справочники.X.ПроверитьСовпадениеПоРеквизиту).
func TestMatchCatalogByField(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	entity := &metadata.Entity{
		Name: "Контрагент",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "ИНН", Type: metadata.FieldTypeString},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{entity}); err != nil {
		t.Fatal(err)
	}

	write := func(name, inn string) {
		if _, err := db.WriteCatalogRecord(ctx, entity, "", map[string]any{
			"наименование": name, "инн": inn,
		}); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("ООО Ромашка", "7701234567") // уникальный ИНН
	write("ООО Дубль-1", "5009999999") // дубль …
	write("ООО Дубль-2", "5009999999") // … того же ИНН
	write("ООО Дубль-3", "5009999999") // … трижды

	t.Run("НеНайдено", func(t *testing.T) {
		id, _, count, err := db.MatchCatalogByField(ctx, entity, "ИНН", "0000000000")
		if err != nil {
			t.Fatal(err)
		}
		if count != 0 || id != "" {
			t.Errorf("count=%d id=%q, want 0 и пустой id", count, id)
		}
	})

	t.Run("НайденаОдна", func(t *testing.T) {
		id, display, count, err := db.MatchCatalogByField(ctx, entity, "ИНН", "7701234567")
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("count=%d, want 1", count)
		}
		if id == "" {
			t.Error("ожидался id найденной записи")
		}
		if display != "7701234567" {
			t.Errorf("display=%q, want значение реквизита", display)
		}
	})

	t.Run("НайденоНесколько_точныйСчёт", func(t *testing.T) {
		id, _, count, err := db.MatchCatalogByField(ctx, entity, "ИНН", "5009999999")
		if err != nil {
			t.Fatal(err)
		}
		if count != 3 {
			t.Errorf("count=%d, want 3 (точное число дублей)", count)
		}
		if id != "" {
			t.Errorf("id=%q, ожидался пустой при неоднозначности", id)
		}
	})

	t.Run("НеизвестныйРеквизит", func(t *testing.T) {
		if _, _, _, err := db.MatchCatalogByField(ctx, entity, "НетТакого", "x"); err == nil {
			t.Error("ожидалась ошибка для несуществующего реквизита")
		}
	})
}
