package query_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
	"github.com/ivantit66/onebase/internal/storage"
)

// Регресс на issue #462: дата документа и параметр запроса должны сравниваться
// как одинаково сериализованные UTC-моменты даже при не-UTC зоне процесса.
func TestSQLiteDateFieldComparisonWithTimeParam(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(ctx, `
		CREATE TABLE закрытиемесяца (
			номер TEXT,
			месяц TEXT
		)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx,
		"INSERT INTO закрытиемесяца(номер, месяц) VALUES(?, ?)",
		"0001", "2026-06-30T20:59:00Z",
	); err != nil {
		t.Fatal(err)
	}

	moscow := time.FixedZone("Europe/Moscow", 3*60*60)
	boundary := time.Date(2026, 7, 1, 0, 0, 0, 0, moscow) // 2026-06-30T21:00:00Z
	entity := &metadata.Entity{
		Name: "ЗакрытиеМесяца",
		Kind: metadata.KindDocument,
		Fields: []metadata.Field{
			{Name: "Номер", Type: metadata.FieldTypeString},
			{Name: "Месяц", Type: metadata.FieldTypeDate},
		},
	}
	result, err := query.Compile(
		`ВЫБРАТЬ Номер ИЗ Документ.ЗакрытиеМесяца ГДЕ Месяц < &Граница`,
		query.CompileOpts{
			Dialect:  db.Dialect(),
			Entities: []*metadata.Entity{entity},
			Params:   map[string]any{"Граница": boundary},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Args) != 1 || result.Args[0] != "2026-06-30T21:00:00Z" {
		t.Fatalf("параметр даты = %v, want RFC3339 UTC", result.Args)
	}

	var number string
	if err := db.QueryRow(ctx, result.SQL, result.Args...).Scan(&number); err != nil {
		t.Fatalf("запрос не вернул документ 20:59Z перед границей 21:00Z: %v", err)
	}
	if number != "0001" {
		t.Fatalf("номер = %q, want 0001", number)
	}
}
