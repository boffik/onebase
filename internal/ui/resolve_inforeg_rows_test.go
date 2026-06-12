package ui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/storage"
)

// Issue #44: в списке регистра сведений UUID ссылочных измерений/ресурсов
// должны заменяться на наименования, как это происходит в регистрах накопления.
func TestResolveInfoRegRows_RefDimension(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Справочник-измерение
	kontragent := &metadata.Entity{
		Name:   "Контрагент",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{kontragent}); err != nil {
		t.Fatal(err)
	}
	kID := uuid.New()
	if err := db.Upsert(ctx, "Контрагент", kID, map[string]any{"Наименование": "ООО Ромашка"}, kontragent); err != nil {
		t.Fatal(err)
	}

	registry := runtime.NewRegistry()
	registry.Load(runtime.LoadOptions{Entities: []*metadata.Entity{kontragent}})
	s := &Server{store: db, reg: registry}

	ir := &metadata.InfoRegister{
		Name: "ЦеныКонтрагентов",
		Dimensions: []metadata.Field{
			{Name: "Контрагент", Type: "reference:Контрагент", RefEntity: "Контрагент"},
		},
		Resources: []metadata.Field{
			{Name: "Цена", Type: metadata.FieldTypeNumber},
		},
	}

	rows := []map[string]any{
		{"Контрагент": kID.String(), "Цена": "100"},
	}
	s.resolveInfoRegRows(ctx, rows, ir)

	// После фикса #44 UUID сохраняется in-place, наименование — в _label.
	if rows[0]["Контрагент"] == "ООО Ромашка" {
		t.Errorf("UUID измерения был заменён наименованием in-place — форма удаления сломается")
	}
	if rows[0]["Контрагент_label"] != "ООО Ромашка" {
		t.Errorf("лейбл не записан в _label: got %v, want 'ООО Ромашка'", rows[0]["Контрагент_label"])
	}
}

// Issue #44 регрессия: resolveInfoRegRows НЕ должна затирать оригинальный ключ UUID.
// Скрытые поля формы удаления используют сырое значение; если оно заменено на
// наименование — DELETE строит WHERE контрагент_id = 'ООО Ромашка' и молча не работает.
func TestResolveInfoRegRows_OriginalKeyPreserved(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	kontragent := &metadata.Entity{
		Name:   "Контрагент",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{kontragent}); err != nil {
		t.Fatal(err)
	}
	kID := uuid.New()
	if err := db.Upsert(ctx, "Контрагент", kID, map[string]any{"Наименование": "ООО Ромашка"}, kontragent); err != nil {
		t.Fatal(err)
	}

	registry := runtime.NewRegistry()
	registry.Load(runtime.LoadOptions{Entities: []*metadata.Entity{kontragent}})
	s := &Server{store: db, reg: registry}

	ir := &metadata.InfoRegister{
		Name: "ЦеныКонтрагентов",
		Dimensions: []metadata.Field{
			{Name: "Контрагент", Type: "reference:Контрагент", RefEntity: "Контрагент"},
		},
		Resources: []metadata.Field{
			{Name: "Цена", Type: metadata.FieldTypeNumber},
		},
	}

	rows := []map[string]any{
		{"Контрагент": kID.String(), "Цена": "100"},
	}
	s.resolveInfoRegRows(ctx, rows, ir)

	// Оригинальный ключ должен оставаться UUID — иначе форма удаления сломана.
	if rows[0]["Контрагент"] != kID.String() {
		t.Errorf("сырое значение измерения перезаписано: got %v, want UUID %v", rows[0]["Контрагент"], kID.String())
	}
	// Лейбл должен быть доступен по ключу с суффиксом _label.
	if rows[0]["Контрагент_label"] != "ООО Ромашка" {
		t.Errorf("лейбл не записан: got %v, want 'ООО Ромашка'", rows[0]["Контрагент_label"])
	}
}
