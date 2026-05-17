package interpreter_test

import (
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
)

// runReturning compiles src, runs procedure "Test" and returns the value
// assigned to a global captured via the "Out" sink. Used by Plan 36 smoke
// tests to verify lexer/parser/interpreter wiring for new DSL features.
func runReturning(t *testing.T, src string) any {
	t.Helper()
	l := lexer.New(src, "smoke.os")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Procedures) == 0 {
		t.Fatalf("no procedures")
	}
	interp := interpreter.New()
	var captured any
	sink := interpreter.BuiltinFunc(func(args []any, file string, line int) (any, error) {
		if len(args) > 0 {
			captured = args[0]
		}
		return nil, nil
	})
	extra := map[string]any{"out": sink, "Out": sink}
	if err := interp.Run(prog.Procedures[0], nil, extra); err != nil {
		t.Fatalf("run: %v", err)
	}
	return captured
}

func TestPlan36_ElseIf(t *testing.T) {
	src := `Процедура Test()
  x = 2
  Если x = 1 Тогда
    Out("one")
  ИначеЕсли x = 2 Тогда
    Out("two")
  Иначе
    Out("three")
  КонецЕсли
КонецПроцедуры`
	if got := runReturning(t, src); got != "two" {
		t.Errorf("ИначеЕсли: got %v, want two", got)
	}
}

func TestPlan36_Ternary(t *testing.T) {
	src := `Процедура Test()
  x = ?(Истина, "yes", "no")
  Out(x)
КонецПроцедуры`
	if got := runReturning(t, src); got != "yes" {
		t.Errorf("Тернарный: got %v, want yes", got)
	}
}

func TestPlan36_CompoundAssignPlus(t *testing.T) {
	src := `Процедура Test()
  x = 5
  x += 3
  Out(x)
КонецПроцедуры`
	got := runReturning(t, src)
	if f, ok := got.(float64); !ok || f != 8 {
		t.Errorf("+=: got %v (%T), want 8.0", got, got)
	}
}

func TestPlan36_CompoundAssignMul(t *testing.T) {
	src := `Процедура Test()
  x = 4
  x *= 2.5
  Out(x)
КонецПроцедуры`
	got := runReturning(t, src)
	if f, ok := got.(float64); !ok || f != 10 {
		t.Errorf("*=: got %v (%T), want 10.0", got, got)
	}
}

func TestPlan36_DateBegMonth(t *testing.T) {
	src := `Процедура Test()
  d = НачалоМесяца(ТекущаяДата())
  Out(d)
КонецПроцедуры`
	got := runReturning(t, src)
	tt, ok := got.(time.Time)
	if !ok {
		t.Fatalf("НачалоМесяца: got %T, want time.Time", got)
	}
	if tt.Day() != 1 || tt.Hour() != 0 || tt.Minute() != 0 {
		t.Errorf("НачалоМесяца: %v — должен быть 1-е число, 00:00", tt)
	}
}

func TestPlan36_DateAddMonth(t *testing.T) {
	src := `Процедура Test()
  d = ДобавитьМесяц(НачалоМесяца(ТекущаяДата()), 2)
  Out(Месяц(d))
КонецПроцедуры`
	got := runReturning(t, src)
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("ДобавитьМесяц: got %T", got)
	}
	now := time.Now()
	expected := (int(now.Month())+1)%12 + 1
	if int(f) != expected {
		t.Errorf("ДобавитьМесяц: got month %v, want %v", f, expected)
	}
}

func TestPlan36_StrReplace(t *testing.T) {
	src := `Процедура Test()
  Out(СтрЗаменить("hello world", "world", "OneBase"))
КонецПроцедуры`
	if got := runReturning(t, src); got != "hello OneBase" {
		t.Errorf("СтрЗаменить: got %q", got)
	}
}

func TestPlan36_StrContains(t *testing.T) {
	src := `Процедура Test()
  Out(СтрСодержит("abcdef", "cd"))
КонецПроцедуры`
	if got := runReturning(t, src); got != true {
		t.Errorf("СтрСодержит: got %v, want true", got)
	}
}

func TestPlan36_StrTemplate(t *testing.T) {
	src := `Процедура Test()
  Out(СтрШаблон("Привет, %1, тебе %2 лет", "Иван", 30))
КонецПроцедуры`
	want := "Привет, Иван, тебе 30 лет"
	if got := runReturning(t, src); got != want {
		t.Errorf("СтрШаблон: got %q, want %q", got, want)
	}
}

func TestPlan36_IsBlank(t *testing.T) {
	src := `Процедура Test()
  Out(Пустая(""))
КонецПроцедуры`
	if got := runReturning(t, src); got != true {
		t.Errorf("Пустая(\"\"): got %v", got)
	}
}

func TestPlan36_IsFilled(t *testing.T) {
	src := `Процедура Test()
  Out(ЗначениеЗаполнено("hello"))
КонецПроцедуры`
	if got := runReturning(t, src); got != true {
		t.Errorf("ЗначениеЗаполнено: got %v", got)
	}
}

func TestPlan36_TypeOf(t *testing.T) {
	src := `Процедура Test()
  Out(ТипЗнч(42))
КонецПроцедуры`
	if got := runReturning(t, src); got != "Число" {
		t.Errorf("ТипЗнч(42): got %q", got)
	}
}

func TestPlan36_TypeCompare(t *testing.T) {
	src := `Процедура Test()
  Out(ТипЗнч("hello") = Тип("Строка"))
КонецПроцедуры`
	if got := runReturning(t, src); got != true {
		t.Errorf("ТипЗнч = Тип: got %v", got)
	}
}

func TestPlan36_Format_Number(t *testing.T) {
	src := `Процедура Test()
  Out(Формат(1234.567, "ЧДЦ=2; ЧРГ=' '"))
КонецПроцедуры`
	got := runReturning(t, src)
	// Expect "1 234.57" — space thousands separator, 2 decimals.
	s, ok := got.(string)
	if !ok {
		t.Fatalf("Формат: got %T", got)
	}
	if s != "1 234.57" {
		t.Errorf("Формат число: got %q, want %q", s, "1 234.57")
	}
}
