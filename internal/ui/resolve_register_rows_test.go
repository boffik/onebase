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

func TestAsString(t *testing.T) {
	if asString("привет") != "привет" {
		t.Error("string не прошёл")
	}
	if asString([]byte("байты")) != "байты" {
		t.Error("[]byte не сконвертировался")
	}
	if asString(nil) != "" {
		t.Error("nil → не пустая строка")
	}
	if asString(42) != "" {
		t.Error("число → должна быть пустая строка")
	}
}

// resolveRegisterRows: UUID в измерении (reference) и атрибуте → имя,
// причём целевым поиском по RefEntity, и поддержка []byte от SQLite-драйвера.
func TestResolveRegisterRows_RefAndBytes(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	nom := &metadata.Entity{
		Name:   "Номенклатура",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	org := &metadata.Entity{
		Name:   "Организация",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{nom, org}); err != nil {
		t.Fatal(err)
	}
	nomID := uuid.New()
	orgID := uuid.New()
	if err := db.Upsert(ctx, "Номенклатура", nomID, map[string]any{"Наименование": "Тумбочка"}, nom); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert(ctx, "Организация", orgID, map[string]any{"Наименование": "ООО Ромашка"}, org); err != nil {
		t.Fatal(err)
	}

	registry := runtime.NewRegistry()
	registry.Load([]*metadata.Entity{nom, org}, nil, nil, nil, nil, nil, nil)
	s := &Server{store: db, reg: registry}

	reg := &metadata.Register{
		Name: "ОстаткиТоваров",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: "reference:Номенклатура", RefEntity: "Номенклатура"},
		},
		Attributes: []metadata.Field{
			{Name: "Организация", Type: "reference:Организация", RefEntity: "Организация"},
		},
	}

	rows := []map[string]any{
		// измерение — строка-UUID, атрибут — []byte-UUID (как может вернуть SQLite)
		{"Номенклатура": nomID.String(), "Организация": []byte(orgID.String())},
	}
	s.resolveRegisterRows(ctx, rows, reg)

	if rows[0]["Номенклатура"] != "Тумбочка" {
		t.Errorf("измерение не резолвнулось: %v", rows[0]["Номенклатура"])
	}
	if rows[0]["Организация"] != "ООО Ромашка" {
		t.Errorf("атрибут ([]byte UUID) не резолвнулся: %v", rows[0]["Организация"])
	}
}

// Legacy string-измерение, хранящее UUID без RefEntity, тоже резолвится
// (через скан всех сущностей как fallback).
func TestResolveRegisterRows_LegacyStringUUID(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	skl := &metadata.Entity{
		Name:   "Склад",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{skl}); err != nil {
		t.Fatal(err)
	}
	sklID := uuid.New()
	if err := db.Upsert(ctx, "Склад", sklID, map[string]any{"Наименование": "Основной"}, skl); err != nil {
		t.Fatal(err)
	}
	registry := runtime.NewRegistry()
	registry.Load([]*metadata.Entity{skl}, nil, nil, nil, nil, nil, nil)
	s := &Server{store: db, reg: registry}

	reg := &metadata.Register{
		Name: "ОстаткиТоваров",
		// Склад как string (workaround П.17), но хранит UUID
		Dimensions: []metadata.Field{{Name: "Склад", Type: metadata.FieldTypeString}},
	}
	rows := []map[string]any{{"Склад": sklID.String()}}
	s.resolveRegisterRows(ctx, rows, reg)

	if rows[0]["Склад"] != "Основной" {
		t.Errorf("legacy string-UUID не резолвнулся: %v", rows[0]["Склад"])
	}
}
