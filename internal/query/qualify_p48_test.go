package query_test

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
	"github.com/ivantit66/onebase/internal/storage"
)

// п.48: при авто-JOIN ссылочного поля собственная колонка основной таблицы
// должна квалифицироваться именем таблицы, иначе одноимённая колонка
// присоединённого каталога даёт ambiguous column.
func TestQualifyOwnColumn_UnderRefJoin(t *testing.T) {
	ent := &metadata.Entity{
		Name: "Заказ",
		Kind: metadata.KindDocument,
		Fields: []metadata.Field{
			{Name: "Контрагент", Type: "reference:Контрагент", RefEntity: "Контрагент"},
			{Name: "Статус", Type: metadata.FieldTypeString},
		},
	}
	src := `ВЫБРАТЬ Статус ИЗ Документ.Заказ ГДЕ Контрагент = &К`
	r, err := query.Compile(src, query.CompileOpts{
		Entities: []*metadata.Entity{ent},
		Params:   map[string]any{"К": "x"},
		Dialect:  storage.SQLiteDialect{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "заказ.статус") {
		t.Errorf("ожидалась квалификация заказ.статус при ref-JOIN:\n%s", r.SQL)
	}
}
