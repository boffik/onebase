package query_test

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
	"github.com/ivantit66/onebase/internal/storage"
)

// п.49: number-поля на SQLite хранятся как TEXT (точный decimal), поэтому в
// сравнениях/сортировке без CAST идёт строковое сравнение («9» > «100»).
// Компилятор должен оборачивать number-колонку в CAST(... AS NUMERIC) — но
// только на SQLite и только в WHERE/HAVING/ORDER BY, не в ВЫБРАТЬ.
func TestNumberCast_SQLiteVsPostgres(t *testing.T) {
	ent := &metadata.Entity{
		Name: "Номенклатура",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "МинимальныйОстаток", Type: metadata.FieldTypeNumber},
		},
	}
	src := `ВЫБРАТЬ Наименование, МинимальныйОстаток ИЗ Справочник.Номенклатура ` +
		`ГДЕ МинимальныйОстаток < 100 УПОРЯДОЧИТЬ ПО МинимальныйОстаток`

	// SQLite — CAST в ГДЕ и УПОРЯДОЧИТЬ, но не в ВЫБРАТЬ.
	r, err := query.Compile(src, query.CompileOpts{
		Entities: []*metadata.Entity{ent},
		Dialect:  storage.SQLiteDialect{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(r.SQL, "CAST(минимальныйостаток AS NUMERIC)"); n != 2 {
		t.Errorf("SQLite: ожидалось 2 CAST (ГДЕ+УПОРЯДОЧИТЬ), got %d:\n%s", n, r.SQL)
	}
	// В ВЫБРАТЬ колонка остаётся без CAST (точный TEXT-вывод).
	selectPart := r.SQL[:strings.Index(r.SQL, "WHERE")]
	if strings.Contains(selectPart, "CAST(") {
		t.Errorf("SQLite: ВЫБРАТЬ не должен содержать CAST:\n%s", selectPart)
	}

	// Postgres — настоящий numeric, CAST не нужен.
	r2, err := query.Compile(src, query.CompileOpts{
		Entities: []*metadata.Entity{ent},
		Dialect:  storage.PgDialect{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r2.SQL, "CAST(") {
		t.Errorf("Postgres: CAST не требуется:\n%s", r2.SQL)
	}
}

// п.49 (алиас-форма — реальный кейс widgets/товары_к_заказу): число с алиасом
// `Н.МинимальныйОстаток` тоже должно оборачиваться в CAST целиком.
func TestNumberCast_AliasedColumn(t *testing.T) {
	ent := &metadata.Entity{
		Name: "Номенклатура",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "МинимальныйОстаток", Type: metadata.FieldTypeNumber},
		},
	}
	src := `ВЫБРАТЬ Н.Наименование ИЗ Справочник.Номенклатура КАК Н ГДЕ Н.МинимальныйОстаток < 100`
	r, err := query.Compile(src, query.CompileOpts{
		Entities: []*metadata.Entity{ent},
		Dialect:  storage.SQLiteDialect{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "CAST(н.минимальныйостаток AS NUMERIC)") {
		t.Errorf("ожидался CAST вокруг алиас-колонки:\n%s", r.SQL)
	}
}
