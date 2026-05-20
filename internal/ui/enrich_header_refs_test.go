package ui

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

// П.37: ссылочные поля ШАПКИ при проведении должны обогащаться до *Ref,
// чтобы Строка(this.Склад) давало имя, а не UUID (симметрично строкам ТЧ).
func TestEnrichHeaderRefs_UUIDToRef(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sklad := &metadata.Entity{
		Name:   "Склад",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	doc := &metadata.Entity{
		Name: "ПоступлениеТоваров",
		Kind: metadata.KindDocument,
		Fields: []metadata.Field{
			{Name: "Номер", Type: metadata.FieldTypeString},
			{Name: "Склад", Type: "reference:Склад", RefEntity: "Склад"},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{sklad, doc}); err != nil {
		t.Fatal(err)
	}

	// записываем склад
	skladID := uuid.New()
	if err := db.Upsert(ctx, "Склад", skladID, map[string]any{"Наименование": "Основной"}, sklad); err != nil {
		t.Fatal(err)
	}

	registry := runtime.NewRegistry()
	registry.Load([]*metadata.Entity{sklad, doc}, nil, nil, nil, nil, nil, nil)
	s := &Server{store: db, reg: registry}

	// шапка документа: Склад приходит сырым UUID-строкой (как из формы)
	obj := &runtime.Object{
		ID:     uuid.New(),
		Type:   "ПоступлениеТоваров",
		Kind:   metadata.KindDocument,
		Fields: map[string]any{"номер": "ПОС-1", "склад": skladID.String()},
	}

	s.enrichHeaderRefs(ctx, doc, obj)

	v := obj.Get("Склад")
	ref, ok := v.(*interpreter.Ref)
	if !ok {
		t.Fatalf("Склад → %T, ожидался *interpreter.Ref", v)
	}
	if ref.UUID != skladID.String() {
		t.Errorf("UUID не сохранился: %s", ref.UUID)
	}
	if ref.Name != "Основной" {
		t.Errorf("Name = %q, ожидалось «Основной» (Строка() дал бы имя)", ref.Name)
	}
	// не-ссылочное поле не трогаем
	if obj.Get("Номер") != "ПОС-1" {
		t.Errorf("номер изменился: %v", obj.Get("Номер"))
	}
}

// Поле, уже являющееся *Ref (проведение из обработки), не должно
// перезаписываться — двойной обработки нет.
func TestEnrichHeaderRefs_SkipsExistingRef(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sklad := &metadata.Entity{
		Name:   "Склад",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	doc := &metadata.Entity{
		Name: "ПоступлениеТоваров",
		Kind: metadata.KindDocument,
		Fields: []metadata.Field{
			{Name: "Склад", Type: "reference:Склад", RefEntity: "Склад"},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{sklad, doc}); err != nil {
		t.Fatal(err)
	}
	registry := runtime.NewRegistry()
	registry.Load([]*metadata.Entity{sklad, doc}, nil, nil, nil, nil, nil, nil)
	s := &Server{store: db, reg: registry}

	orig := &interpreter.Ref{UUID: uuid.New().String(), Name: "ИзОбработки", Type: "Склад"}
	obj := &runtime.Object{
		ID:     uuid.New(),
		Type:   "ПоступлениеТоваров",
		Kind:   metadata.KindDocument,
		Fields: map[string]any{"склад": orig},
	}
	s.enrichHeaderRefs(ctx, doc, obj)

	v := obj.Get("Склад")
	ref, ok := v.(*interpreter.Ref)
	if !ok {
		t.Fatalf("Склад → %T, ожидался *interpreter.Ref", v)
	}
	if ref != orig {
		t.Error("существующий *Ref был перезаписан — должен пропускаться")
	}
}
