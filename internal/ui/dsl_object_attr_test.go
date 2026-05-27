package ui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/storage"
)

// ЗначениеРеквизитаОбъекта читает реквизит по ссылке: скалярный — как есть,
// ссылочный — как ссылку (для цепочек). Неизвестный реквизит → ошибка.
func TestObjectAttributeValue(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	контрагент := &metadata.Entity{
		Name: "Контрагент", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	номенклатура := &metadata.Entity{
		Name: "Номенклатура", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "СтавкаНДС", Type: metadata.FieldTypeNumber},
			{Name: "Поставщик", Type: "reference:Контрагент", RefEntity: "Контрагент"},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{контрагент, номенклатура}); err != nil {
		t.Fatal(err)
	}

	registry := runtime.NewRegistry()
	registry.Load(runtime.LoadOptions{Entities: []*metadata.Entity{контрагент, номенклатура}})
	s := &Server{store: db, reg: registry}

	catalogs := interpreter.NewCatalogsRoot(interpreter.NewStaticCtx(ctx), db, registry)
	create := func(entityName string, set map[string]any) *interpreter.Ref {
		t.Helper()
		cp := catalogs.Get(entityName).(*interpreter.CatalogProxy)
		w := cp.CallMethod("создать", nil).(*interpreter.CatalogRecordWriter)
		for k, v := range set {
			w.Set(k, v)
		}
		return w.CallMethod("записать", nil).(*interpreter.Ref)
	}

	постРеф := create("Контрагент", map[string]any{"Наименование": "ООО Поставщик"})
	номРеф := create("Номенклатура", map[string]any{
		"Наименование": "Стул",
		"СтавкаНДС":    float64(20),
		"Поставщик":    постРеф,
	})

	// Скалярный реквизит (число).
	v, err := s.objectAttributeValue(ctx, []any{номРеф, "СтавкаНДС"})
	if err != nil {
		t.Fatalf("СтавкаНДС: %v", err)
	}
	if v != float64(20) {
		t.Errorf("СтавкаНДС = %v (%T), ожидалось 20", v, v)
	}

	// Ссылочный реквизит → *Ref с наименованием и типом.
	pv, err := s.objectAttributeValue(ctx, []any{номРеф, "Поставщик"})
	if err != nil {
		t.Fatalf("Поставщик: %v", err)
	}
	pref, ok := pv.(*interpreter.Ref)
	if !ok {
		t.Fatalf("Поставщик → %T, ожидался *interpreter.Ref", pv)
	}
	if pref.Name != "ООО Поставщик" || pref.Type != "Контрагент" {
		t.Errorf("ссылка-реквизит: name=%q type=%q", pref.Name, pref.Type)
	}

	// Цепочка: реквизит ссылки, полученной из реквизита.
	chained, err := s.objectAttributeValue(ctx, []any{pref, "Наименование"})
	if err != nil {
		t.Fatalf("цепочка: %v", err)
	}
	if chained != "ООО Поставщик" {
		t.Errorf("цепочка дала %q, ожидалось «ООО Поставщик»", chained)
	}

	// Неизвестный реквизит → ошибка, а не тихий nil.
	if _, err := s.objectAttributeValue(ctx, []any{номРеф, "НетТакого"}); err == nil {
		t.Error("ожидалась ошибка для несуществующего реквизита")
	}

	// Пустой/nil первый аргумент → nil без ошибки.
	if v, err := s.objectAttributeValue(ctx, []any{nil, "СтавкаНДС"}); v != nil || err != nil {
		t.Errorf("nil-ссылка: v=%v err=%v, ожидалось nil/nil", v, err)
	}
	empty := &interpreter.Ref{Type: "Номенклатура"}
	if v, err := s.objectAttributeValue(ctx, []any{empty, "СтавкаНДС"}); v != nil || err != nil {
		t.Errorf("пустая ссылка: v=%v err=%v, ожидалось nil/nil", v, err)
	}
}
