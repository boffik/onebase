package interpreter_test

import (
	"testing"

	"github.com/boffik/onebase/internal/dsl/interpreter"
	"github.com/boffik/onebase/internal/dsl/lexer"
	"github.com/boffik/onebase/internal/dsl/parser"
	"github.com/boffik/onebase/internal/metadata"
	"github.com/boffik/onebase/internal/runtime"
)

func runFunc(t *testing.T, src string) any {
	t.Helper()
	l := lexer.New(src, "test.os")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	interp := interpreter.New()
	obj := runtime.NewObject("Test", metadata.KindDocument)
	var result any
	_ = interp.RunWithResult(prog.Procedures[0], obj, &result)
	return result
}

func TestErrorInfo_Description(t *testing.T) {
	src := `Функция Тест()
  Попытка
    ВызватьИсключение("моя ошибка");
  Исключение
    Инфо = ИнформацияОбОшибке();
    Возврат Инфо.Описание;
  КонецПопытки;
  Возврат "";
КонецФункции`
	if got := runFunc(t, src); got != "моя ошибка" {
		t.Fatalf("Описание: got %v", got)
	}
}

func TestErrorInfo_LineAndSource(t *testing.T) {
	src := `Функция Тест()
  Попытка
    ВызватьИсключение("x");
  Исключение
    Инфо = ИнформацияОбОшибке();
    Возврат Инфо.НомерСтроки;
  КонецПопытки;
  Возврат 0;
КонецФункции`
	// ВызватьИсключение на 3-й строке исходника.
	if got := runFunc(t, src); got != float64(3) {
		t.Fatalf("НомерСтроки: ожидалось 3, got %v", got)
	}

	srcSrc := `Функция Тест()
  Попытка
    ВызватьИсключение("x");
  Исключение
    Возврат ИнформацияОбОшибке().Источник;
  КонецПопытки;
  Возврат "";
КонецФункции`
	if got := runFunc(t, srcSrc); got != "test.os" {
		t.Fatalf("Источник: got %v", got)
	}
}

func TestErrorInfo_PropertyMethod(t *testing.T) {
	// Доступ через Свойство() тоже работает (Структура).
	src := `Функция Тест()
  Попытка
    ВызватьИсключение("через свойство");
  Исключение
    Возврат ИнформацияОбОшибке().Свойство("Описание");
  КонецПопытки;
  Возврат "";
КонецФункции`
	if got := runFunc(t, src); got != "через свойство" {
		t.Fatalf("Свойство(Описание): got %v", got)
	}
}
