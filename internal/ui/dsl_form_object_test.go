package ui

import (
	"testing"

	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
)

// План 37, этап 8: formObjectThis должен возвращать formTpProxy при Get
// по имени ТЧ, чтобы DSL-выражение Объект.Товары.Добавить() реально
// модифицировало obj.TablePartRows.
func TestFormObjectThis_GetReturnsTpProxy(t *testing.T) {
	entity := &metadata.Entity{
		Name: "Документ",
		TableParts: []metadata.TablePart{
			{Name: "Товары", Fields: []metadata.Field{
				{Name: "Количество", Type: metadata.FieldTypeNumber},
			}},
		},
	}
	obj := &runtime.Object{
		ID:            uuid.New(),
		Type:          entity.Name,
		Fields:        map[string]any{},
		TablePartRows: map[string][]map[string]any{},
	}
	this := &formObjectThis{obj: obj, entity: entity}

	got := this.Get("Товары")
	tp, ok := got.(*formTpProxy)
	if !ok {
		t.Fatalf("Get(\"Товары\") = %T, ожидался *formTpProxy", got)
	}
	if tp.tpName != "Товары" {
		t.Errorf("tpName = %q, ожидалось \"Товары\"", tp.tpName)
	}
}

// formTpProxy.Добавить добавляет строку в obj.TablePartRows и возвращает
// *interpreter.MapThis, в которую DSL присвоит .Количество и .Цена.
func TestFormTpProxy_AddRowModifiesObject(t *testing.T) {
	obj := &runtime.Object{
		TablePartRows: map[string][]map[string]any{},
	}
	tp := &formTpProxy{obj: obj, tpName: "Товары"}

	res := tp.CallMethod("добавить", nil)
	row, ok := res.(*interpreter.MapThis)
	if !ok {
		t.Fatalf("CallMethod(\"добавить\") = %T, ожидался *MapThis", res)
	}
	row.Set("Количество", float64(5))

	if len(obj.TablePartRows["Товары"]) != 1 {
		t.Fatalf("ожидалась 1 строка ТЧ, получено %d", len(obj.TablePartRows["Товары"]))
	}
	// MapThis.Set делает strings.ToLower(name) — проверяем по lowercase ключу.
	if obj.TablePartRows["Товары"][0]["количество"] != float64(5) {
		t.Errorf("после Set(\"Количество\", 5) row[количество] = %v, ожидалось 5", obj.TablePartRows["Товары"][0]["количество"])
	}
}

// Очистить должен сбросить ТЧ к nil (а не оставить старые строки).
func TestFormTpProxy_Clear(t *testing.T) {
	obj := &runtime.Object{
		TablePartRows: map[string][]map[string]any{
			"Товары": {{"x": 1}, {"x": 2}},
		},
	}
	tp := &formTpProxy{obj: obj, tpName: "Товары"}
	tp.CallMethod("очистить", nil)
	if len(obj.TablePartRows["Товары"]) != 0 {
		t.Errorf("после Очистить() ожидался пустой список, получено %d строк", len(obj.TablePartRows["Товары"]))
	}
}

// Количество возвращает float64 (DSL числа — float64) — без этого
// сравнение `Объект.Товары.Количество() = 0` всегда было бы false.
func TestFormTpProxy_Count(t *testing.T) {
	obj := &runtime.Object{
		TablePartRows: map[string][]map[string]any{
			"Товары": {{"x": 1}, {"x": 2}, {"x": 3}},
		},
	}
	tp := &formTpProxy{obj: obj, tpName: "Товары"}
	got := tp.CallMethod("количество", nil)
	if got != float64(3) {
		t.Errorf("Количество() = %v (%T), ожидалось 3.0", got, got)
	}
}

// IterateRows — для `Для Каждого Стр Из Объект.Товары Цикл` интерпретатор
// должен видеть []map[string]any. Этот тест защищает интерфейс от случайной
// сигнатуры (например, если поменяют на []any).
func TestFormTpProxy_IterateRows(t *testing.T) {
	rows := []map[string]any{{"a": 1}, {"a": 2}}
	obj := &runtime.Object{TablePartRows: map[string][]map[string]any{"Товары": rows}}
	tp := &formTpProxy{obj: obj, tpName: "Товары"}

	got := tp.IterateRows()
	if len(got) != 2 || got[0]["a"] != 1 || got[1]["a"] != 2 {
		t.Errorf("IterateRows() = %v, ожидалось %v", got, rows)
	}
}
