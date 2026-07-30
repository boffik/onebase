package interpreter

import (
	"testing"

	"github.com/shopspring/decimal"
)

type recorderStub struct{ outcomes []AssertOutcome }

func (r *recorderStub) RecordAssert(o AssertOutcome) { r.outcomes = append(r.outcomes, o) }

func TestAssertRoot_Methods(t *testing.T) {
	rec := &recorderStub{}
	a := NewAssertRoot(rec)

	cases := []struct {
		name   string
		method string
		args   []any
		want   bool
	}{
		// Числа сравниваются по значению: decimal(4) == int64(4).
		{"равно-число", "Равно", []any{decimal.NewFromInt(4), int64(4), "четыре"}, true},
		{"равно-строка", "равно", []any{"7999", "7999", "телефон"}, true},
		{"равно-провал", "Равно", []any{"a", "b", "разные"}, false},
		{"неравно", "НеРавно", []any{"a", "b", "разные"}, true},
		{"неравно-провал", "неравно", []any{"a", "a", "одинаковые"}, false},
		{"истина", "Истина", []any{true, "usl"}, true},
		{"истина-провал", "Истина", []any{false, "usl"}, false},
		{"ложь", "Ложь", []any{false, "usl"}, true},
		{"заполнено", "Заполнено", []any{"x", "z"}, true},
		{"заполнено-пусто", "Заполнено", []any{"", "z"}, false},
		{"провалить", "Провалить", []any{"всегда"}, false},
	}

	for i, c := range cases {
		got := a.CallMethod(c.method, c.args)
		gotBool, ok := got.(bool)
		if !ok {
			t.Fatalf("%s: метод вернул не Булево: %T", c.name, got)
		}
		if gotBool != c.want {
			t.Fatalf("%s: результат %v, ожидался %v", c.name, gotBool, c.want)
		}
		if rec.outcomes[i].Passed != c.want {
			t.Fatalf("%s: записан Passed=%v, ожидался %v", c.name, rec.outcomes[i].Passed, c.want)
		}
		// Провалившиеся проверки должны нести деталь для отчёта.
		if !c.want && rec.outcomes[i].Detail == "" {
			t.Fatalf("%s: у провала должна быть непустая деталь", c.name)
		}
	}
}

func TestAssertRoot_UnknownMethodPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("неизвестный метод должен паниковать userError")
		}
	}()
	NewAssertRoot(&recorderStub{}).CallMethod("НетТакого", nil)
}

func TestAssertRoot_NilRecorderSafe(t *testing.T) {
	a := NewAssertRoot(nil)
	if got := a.CallMethod("Истина", []any{true, "ok"}); got != true {
		t.Fatalf("с nil-рекордером метод должен вернуть результат, получено %v", got)
	}
}
