package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

func TestListParamsRowFilterAppliesToListAndCount(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "rls.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	cat := &metadata.Entity{
		Name: "Товар",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Owner", Type: metadata.FieldTypeString},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{cat}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := db.Upsert(ctx, cat.Name, uuid.New(), map[string]any{"Наименование": "A", "Owner": "u1"}, cat); err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	if err := db.Upsert(ctx, cat.Name, uuid.New(), map[string]any{"Наименование": "B", "Owner": "u2"}, cat); err != nil {
		t.Fatalf("upsert B: %v", err)
	}
	params := ListParams{RowFilter: &Predicate{Field: "Owner", Op: "eq", Value: "u1"}}
	rows, err := db.List(ctx, cat.Name, cat, params)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 || rows[0]["Owner"] != "u1" {
		t.Fatalf("rows = %#v", rows)
	}
	total, err := db.CountList(ctx, cat.Name, cat, params)
	if err != nil {
		t.Fatalf("CountList: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
}

func TestReferencePredicateAppliesToListCountAndMemoryMatch(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "rls-ref.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	client := &metadata.Entity{
		Name: "Клиент",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Owner", Type: metadata.FieldTypeString},
		},
	}
	order := &metadata.Entity{
		Name: "Заказ",
		Kind: metadata.KindDocument,
		Fields: []metadata.Field{
			{Name: "Номер", Type: metadata.FieldTypeString},
			{Name: "Клиент", Type: metadata.FieldTypeString, RefEntity: client.Name},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{client, order}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	allowedClient := uuid.New()
	hiddenClient := uuid.New()
	if err := db.Upsert(ctx, client.Name, allowedClient, map[string]any{"Наименование": "A", "Owner": "u1"}, client); err != nil {
		t.Fatalf("upsert allowed client: %v", err)
	}
	if err := db.Upsert(ctx, client.Name, hiddenClient, map[string]any{"Наименование": "B", "Owner": "u2"}, client); err != nil {
		t.Fatalf("upsert hidden client: %v", err)
	}
	allowedOrder := uuid.New()
	hiddenOrder := uuid.New()
	if err := db.Upsert(ctx, order.Name, allowedOrder, map[string]any{"Номер": "1", "Клиент": allowedClient.String()}, order); err != nil {
		t.Fatalf("upsert allowed order: %v", err)
	}
	if err := db.Upsert(ctx, order.Name, hiddenOrder, map[string]any{"Номер": "2", "Клиент": hiddenClient.String()}, order); err != nil {
		t.Fatalf("upsert hidden order: %v", err)
	}
	pred := &Predicate{
		Field:     "Клиент",
		RefEntity: client,
		RefPredicate: &Predicate{
			Field: "Owner",
			Op:    "eq",
			Value: "u1",
		},
	}
	params := ListParams{RowFilter: pred}
	rows, err := db.List(ctx, order.Name, order, params)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 || rows[0]["Номер"] != "1" {
		t.Fatalf("rows = %#v", rows)
	}
	total, err := db.CountList(ctx, order.Name, order, params)
	if err != nil {
		t.Fatalf("CountList: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	row, err := db.GetByID(ctx, order.Name, allowedOrder, order)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !MatchPredicateWithRefs(row, pred, func(entity *metadata.Entity, id uuid.UUID) (map[string]any, bool) {
		refRow, err := db.GetByID(ctx, entity.Name, id, entity)
		return refRow, err == nil
	}) {
		t.Fatalf("allowed row must match reference predicate: %#v", row)
	}
}

func TestPredicateValuesEqualDoesNotCoerceNumbersThroughBool(t *testing.T) {
	if valuesEqual(7, 1) {
		t.Fatal("7 must not equal 1 through bool coercion")
	}
	if !valuesEqual(1, 1.0) {
		t.Fatal("numeric equality must still compare numeric values")
	}
	if valuesEqual("да", "yes") {
		t.Fatal("strings must not be compared as bool aliases")
	}
	if !valuesEqual(int64(1), true) {
		t.Fatal("DB bool representation int64(1) must match true")
	}
}

func TestPredicateSQLRejectsScalarInNotIn(t *testing.T) {
	cat := &metadata.Entity{
		Name:   "Товар",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Owner", Type: metadata.FieldTypeString}},
	}
	_, _, _, err := PredicateSQL(SQLiteDialect{}, cat, &Predicate{Field: "Owner", Op: "not_in", Value: "u"}, 1)
	if err == nil {
		t.Fatal("scalar not_in must fail closed")
	}
}

func TestRegFilterRowFilterAppliesBeforeMovementsAndBalances(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "reg-rls.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	reg := &metadata.Register{
		Name: "Остатки",
		Dimensions: []metadata.Field{
			{Name: "Owner", Type: metadata.FieldTypeString},
		},
		Resources: []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
	}
	if err := db.MigrateRegisters(ctx, []*metadata.Register{reg}); err != nil {
		t.Fatalf("MigrateRegisters: %v", err)
	}
	period := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	if err := db.WriteMovements(ctx, reg.Name, "Док", uuid.New(), []map[string]any{
		{"Owner": "u1", "Количество": 10},
		{"Owner": "u2", "Количество": 20},
	}, reg, &period); err != nil {
		t.Fatalf("WriteMovements: %v", err)
	}
	filter := RegFilter{RowFilter: &Predicate{Field: "Owner", Op: "eq", Value: "u1"}}
	movements, err := db.GetMovements(ctx, reg.Name, reg, filter)
	if err != nil {
		t.Fatalf("GetMovements: %v", err)
	}
	if len(movements) != 1 || movements[0]["Owner"] != "u1" {
		t.Fatalf("movements = %#v", movements)
	}
	balances, err := db.GetBalances(ctx, reg.Name, reg, filter)
	if err != nil {
		t.Fatalf("GetBalances: %v", err)
	}
	if len(balances) != 1 || balances[0]["Owner"] != "u1" {
		t.Fatalf("balances = %#v", balances)
	}
}

func TestInfoRegListRowFilter(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "ir-rls.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	ir := &metadata.InfoRegister{
		Name:       "Настройки",
		Dimensions: []metadata.Field{{Name: "Ключ", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Owner", Type: metadata.FieldTypeString}},
	}
	if err := db.MigrateInfoRegisters(ctx, []*metadata.InfoRegister{ir}); err != nil {
		t.Fatalf("MigrateInfoRegisters: %v", err)
	}
	if err := db.InfoRegSet(ctx, ir, map[string]any{"Ключ": "a"}, map[string]any{"Owner": "u1"}, nil); err != nil {
		t.Fatalf("InfoRegSet a: %v", err)
	}
	if err := db.InfoRegSet(ctx, ir, map[string]any{"Ключ": "b"}, map[string]any{"Owner": "u2"}, nil); err != nil {
		t.Fatalf("InfoRegSet b: %v", err)
	}
	rows, err := db.InfoRegList(ctx, ir, RegFilter{RowFilter: &Predicate{Field: "Owner", Op: "eq", Value: "u1"}})
	if err != nil {
		t.Fatalf("InfoRegList: %v", err)
	}
	if len(rows) != 1 || rows[0]["Owner"] != "u1" {
		t.Fatalf("rows = %#v", rows)
	}
}
