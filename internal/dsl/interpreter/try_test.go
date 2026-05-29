package interpreter_test

import (
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
)

func evalFuncTry(t *testing.T, src string) any {
	t.Helper()
	l := lexer.New(src, "test.os")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Procedures) == 0 {
		t.Fatal("no procedures")
	}
	interp := interpreter.New()
	obj := runtime.NewObject("Test", metadata.KindDocument)
	var result any
	_ = interp.RunWithResult(prog.Procedures[0], obj, &result)
	return result
}

func TestTry_CatchesUserError(t *testing.T) {
	src := `Функция Тест()
  x = 0;
  Попытка
    Error("упс");
  Исключение
    x = 1;
  КонецПопытки;
  Возврат x;
КонецФункции`

	result := evalFuncTry(t, src)
	if !numEq(result, 1) {
		t.Fatalf("expected 1, got %v", result)
	}
}

func TestTry_NoError_SkipsExcept(t *testing.T) {
	src := `Функция Тест()
  x = 0;
  Попытка
    x = 1;
  Исключение
    x = 99;
  КонецПопытки;
  Возврат x;
КонецФункции`

	result := evalFuncTry(t, src)
	if !numEq(result, 1) {
		t.Fatalf("expected 1, got %v", result)
	}
}

func TestTry_ErrorDescription(t *testing.T) {
	src := `Функция Тест()
  Попытка
    Error("моя ошибка");
  Исключение
    Возврат ОписаниеОшибки();
  КонецПопытки;
  Возврат "";
КонецФункции`

	result := evalFuncTry(t, src)
	if result != "моя ошибка" {
		t.Fatalf("expected 'моя ошибка', got %v", result)
	}
}

func TestTry_Nested_InnerCatch(t *testing.T) {
	src := `Функция Тест()
  Попытка
    Попытка
      Error("inner");
    Исключение
      Error("outer-from-inner");
    КонецПопытки;
  Исключение
    Возврат ОписаниеОшибки();
  КонецПопытки;
  Возврат "";
КонецФункции`

	result := evalFuncTry(t, src)
	if result != "outer-from-inner" {
		t.Fatalf("expected 'outer-from-inner', got %v", result)
	}
}

func TestTry_ErrorPropagatesWithoutExcept(t *testing.T) {
	// Без Исключения-блока — ошибка пробрасывается наверх
	src := `Процедура Тест()
  Попытка
    Error("пробрасываю");
  КонецПопытки;
КонецПроцедуры`
	l := lexer.New(src, "test.os")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	interp := interpreter.New()
	obj := runtime.NewObject("Test", metadata.KindDocument)
	runErr := interp.Run(prog.Procedures[0], obj)
	if runErr == nil {
		t.Fatal("expected error to propagate, got nil")
	}
	dslErr, ok := runErr.(*interpreter.DSLError)
	if !ok {
		t.Fatalf("expected DSLError, got %T", runErr)
	}
	if dslErr.Msg != "пробрасываю" {
		t.Fatalf("wrong message: %q", dslErr.Msg)
	}
}

func TestTry_ExecutionContinuesAfterTry(t *testing.T) {
	src := `Функция Тест()
  результат = "";
  Попытка
    Error("ошибка");
  Исключение
    результат = "поймано";
  КонецПопытки;
  Возврат результат + "-продолжение";
КонецФункции`

	result := evalFuncTry(t, src)
	if result != "поймано-продолжение" {
		t.Fatalf("expected 'поймано-продолжение', got %v", result)
	}
}
