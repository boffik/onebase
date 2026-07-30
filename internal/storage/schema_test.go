package storage

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestNewEphemeralSchemaName(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		n := NewEphemeralSchemaName()
		if !strings.HasPrefix(n, "onebase_test_") {
			t.Fatalf("имя %q без ожидаемого префикса", n)
		}
		for _, r := range n {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
			if !ok {
				t.Fatalf("имя %q содержит небезопасный символ %q", n, r)
			}
		}
		if seen[n] {
			t.Fatalf("имя %q повторилось — не уникально", n)
		}
		seen[n] = true
	}
}

// DropSchemaCascade на SQLite — безопасный no-op (нечего чистить).
func TestDropSchemaCascade_SQLiteNoop(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	if err := db.DropSchemaCascade(ctx, "onebase_test_x"); err != nil {
		t.Fatalf("DropSchemaCascade на SQLite должна быть no-op, получено: %v", err)
	}
	if err := db.CreateSchema(ctx, "onebase_test_x"); err == nil {
		t.Fatal("CreateSchema на SQLite должна вернуть ошибку (только PostgreSQL)")
	}
}

// Полный цикл schema-изоляции на живом PostgreSQL: подключение со scoped
// search_path, создание схемы, запись в неё, удаление CASCADE и проверка, что
// схема исчезла. Гейтится на TEST_DATABASE_URL (как прочие PG-тесты).
func TestSchemaIsolation_PG(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	schema := NewEphemeralSchemaName()

	db, err := ConnectWithSchema(ctx, dsn, schema)
	if err != nil {
		t.Fatalf("ConnectWithSchema: %v", err)
	}
	defer db.Close()

	if err := db.CreateSchema(ctx, schema); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	// Неквалифицированная таблица должна лечь в temp-схему (первая в search_path).
	if _, err := db.Exec(ctx, "CREATE TABLE iso_probe(id int)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO iso_probe(id) VALUES (1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var n int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM iso_probe").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("count=%d, ожидалась 1", n)
	}
	// Таблица действительно в нашей схеме, а не в public.
	var reg any
	if err := db.QueryRow(ctx, "SELECT to_regclass($1)", schema+".iso_probe").Scan(&reg); err != nil {
		t.Fatalf("to_regclass: %v", err)
	}
	if reg == nil {
		t.Fatalf("таблица не найдена в схеме %s", schema)
	}

	if err := db.DropSchemaCascade(ctx, schema); err != nil {
		t.Fatalf("DropSchemaCascade: %v", err)
	}
	// После удаления схема отсутствует.
	var exists bool
	if err := db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name=$1)", schema).Scan(&exists); err != nil {
		t.Fatalf("schemata check: %v", err)
	}
	if exists {
		t.Fatalf("схема %s должна быть удалена", schema)
	}
}
