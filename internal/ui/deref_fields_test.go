package ui

import (
	"testing"

	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/runtime"
)

// derefFields возвращает *interpreter.Ref в шапке к UUID-строкам перед записью
// (после enrichHeaderRefs). Тип-assertion сужен именно к *interpreter.Ref, чтобы
// случайно попавший в шапку *runtime.Object — он тоже реализует GetRefUUID
// (object.go: возвращает o.ID.String()) — не превратился молча в UUID-строку:
// это изменило бы семантику поля и могло привести к потере данных при записи.
func TestDerefFields(t *testing.T) {
	refUUID := uuid.New().String()
	embedded := &runtime.Object{
		Type:   "Склад",
		ID:     uuid.New(),
		Fields: map[string]any{"наименование": "Склад основной"},
	}
	obj := &runtime.Object{Fields: map[string]any{
		"Склад":         &interpreter.Ref{UUID: refUUID, Name: "Склад основной", Type: "Склад"},
		"ОбъектКакПоле": embedded,
		"Строка":        "plain-value",
	}}

	derefFields(obj)

	if got, ok := obj.Fields["Склад"].(string); !ok || got != refUUID {
		t.Fatalf("Склад: *interpreter.Ref должен превратиться в UUID %q, got %T=%v",
			refUUID, obj.Fields["Склад"], obj.Fields["Склад"])
	}
	if got, ok := obj.Fields["ОбъектКакПоле"].(*runtime.Object); !ok || got != embedded {
		t.Errorf("ОбъектКакПоле: *runtime.Object не должен дереференситься в UUID "+
			"(assertion сужен до *interpreter.Ref), got %T=%v",
			obj.Fields["ОбъектКакПоле"], obj.Fields["ОбъектКакПоле"])
	}
	if s, ok := obj.Fields["Строка"].(string); !ok || s != "plain-value" {
		t.Errorf("Строка: обычные значения не должны меняться, got %v", obj.Fields["Строка"])
	}
}
