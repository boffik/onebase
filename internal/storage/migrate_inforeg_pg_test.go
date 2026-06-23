//go:build integration

package storage

import (
	"context"
	"os"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

func connectPGForInfoRegMigration(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestMigrateInfoRegisters_PostgresRebuildsPK(t *testing.T) {
	ctx := context.Background()
	db := connectPGForInfoRegMigration(t)

	ir := &metadata.InfoRegister{
		Name:     "PKMigrateTest",
		Periodic: true,
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: "reference:Номенклатура", RefEntity: "Номенклатура"},
			{Name: "ТипЦен", Type: "reference:ТипЦен", RefEntity: "ТипЦен"},
		},
		Resources: []metadata.Field{{Name: "Цена", Type: "number"}},
	}
	table := metadata.InfoRegTableName(ir.Name)
	_, _ = db.Exec(ctx, "DROP TABLE IF EXISTS "+pgQuoteIdent(table))
	_, err := db.Exec(ctx, "CREATE TABLE "+pgQuoteIdent(table)+` (
		номенклатура_id UUID NOT NULL,
		цена NUMERIC,
		PRIMARY KEY (номенклатура_id)
	)`)
	if err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), "DROP TABLE IF EXISTS "+pgQuoteIdent(table)) })

	if err := db.MigrateInfoRegisters(ctx, []*metadata.InfoRegister{ir}); err != nil {
		t.Fatalf("MigrateInfoRegisters: %v", err)
	}

	got, _, err := db.pgPrimaryKey(ctx, table)
	if err != nil {
		t.Fatalf("pgPrimaryKey: %v", err)
	}
	want := pkCols(ir)
	if !stringSlicesEqual(got, want) {
		t.Fatalf("primary key = %v, want %v", got, want)
	}
}
