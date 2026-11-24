package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boffik/onebase/internal/metadata"
)

// #14: предопределённый элемент одного справочника ссылается по имени на
// предопределённый элемент ДРУГОГО справочника. SyncAllPredefined должен
// синхронизировать справочник-цель раньше и резолвить имя в UUID.
func TestSyncAllPredefined_CrossRef(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	org := &metadata.Entity{
		Name: "Организация",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: "string"},
		},
		Predefined: []*metadata.PredefinedItem{
			{Name: "ГоловнойОфис", Fields: map[string]any{"Наименование": "Головной офис"}},
		},
	}
	orgField := metadata.Field{Name: "Организация", Type: "reference:Организация", RefEntity: "Организация"}
	sklad := &metadata.Entity{
		Name: "Склад",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: "string"},
			orgField,
		},
		Predefined: []*metadata.PredefinedItem{
			{Name: "ОсновнойСклад", Fields: map[string]any{
				"Наименование": "Основной склад",
				"Организация":  "ГоловнойОфис", // cross-ref на predefined Организации
			}},
		},
	}

	// Склад объявлен РАНЬШЕ Организации — проверяем, что порядок
	// синхронизации выправляется по зависимостям сам.
	if err := db.Migrate(ctx, []*metadata.Entity{sklad, org}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	orgID, err := db.GetPredefinedID(ctx, "Организация", "ГоловнойОфис")
	if err != nil {
		t.Fatalf("get org predefined: %v", err)
	}

	col := metadata.ColumnName(orgField)
	var stored string
	err = db.QueryRow(ctx,
		`SELECT `+col+` FROM `+metadata.TableName("Склад")+` WHERE _predefined_name = ?`,
		"ОсновнойСклад",
	).Scan(&stored)
	if err != nil {
		t.Fatalf("read склад: %v", err)
	}
	if stored != orgID.String() {
		t.Errorf("cross-ref не резолвился: %s = %q, ожидался UUID ГоловногоОфиса %q",
			col, stored, orgID)
	}
}

// Cross-ref на несуществующий предопределённый элемент → понятная ошибка,
// а не молчаливая вставка имени-строки в UUID-колонку.
func TestSyncAllPredefined_CrossRefMissing(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	org := &metadata.Entity{
		Name: "Организация",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: "string"},
		},
		Predefined: []*metadata.PredefinedItem{
			{Name: "ГоловнойОфис", Fields: map[string]any{"Наименование": "Головной офис"}},
		},
	}
	sklad := &metadata.Entity{
		Name: "Склад",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: "string"},
			{Name: "Организация", Type: "reference:Организация", RefEntity: "Организация"},
		},
		Predefined: []*metadata.PredefinedItem{
			{Name: "ОсновнойСклад", Fields: map[string]any{
				"Наименование": "Основной склад",
				"Организация":  "НетТакойОрганизации",
			}},
		},
	}

	err = db.Migrate(ctx, []*metadata.Entity{org, sklad})
	if err == nil {
		t.Fatal("ожидалась ошибка о ненайденном cross-ref")
	}
	if !strings.Contains(err.Error(), "не найден") {
		t.Errorf("ошибка должна сообщать о ненайденном элементе: %v", err)
	}
}
