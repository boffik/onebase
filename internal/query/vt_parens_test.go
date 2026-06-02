package query_test

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
	"github.com/ivantit66/onebase/internal/storage"
)

// п.47: виртуальная таблица без круглых скобок раньше молча превращалась в имя
// физической таблицы (рантайм `no such table`). Теперь — понятная ошибка компиляции.
func TestVirtualTable_MissingParens(t *testing.T) {
	reg := &metadata.Register{
		Name:       "ЗадачиПоСтатусам",
		Dimensions: []metadata.Field{{Name: "Статус", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
	}
	opts := query.CompileOpts{Registers: []*metadata.Register{reg}, Dialect: storage.SQLiteDialect{}}

	// Без скобок → ошибка
	_, err := query.Compile(`ВЫБРАТЬ Статус ИЗ РегистрНакопления.ЗадачиПоСтатусам.Остатки`, opts)
	if err == nil {
		t.Fatal("ожидалась ошибка для VT без скобок")
	}
	if !strings.Contains(err.Error(), "круглые скобки") {
		t.Errorf("ожидалось понятное сообщение про скобки, got: %v", err)
	}

	// Со скобками → компилируется
	if _, err := query.Compile(`ВЫБРАТЬ Статус, КоличествоОстаток ИЗ РегистрНакопления.ЗадачиПоСтатусам.Остатки()`, opts); err != nil {
		t.Errorf("VT со скобками должна компилироваться, got: %v", err)
	}
}
