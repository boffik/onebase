package interpreter_test

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/shopspring/decimal"
)

// numEq сравнивает числовой результат DSL (теперь decimal.Decimal) с ожидаемым
// значением. Числа в DSL — decimal; служебные счётчики могут быть int64/float64.
func numEq(got any, want float64) bool {
	switch v := got.(type) {
	case decimal.Decimal:
		return v.Equal(decimal.NewFromFloat(want))
	case float64:
		return v == want
	case int64:
		return float64(v) == want
	case int:
		return float64(v) == want
	}
	return false
}

func evalFunc(t *testing.T, src string) any {
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

func evalProgramFunc(t *testing.T, src string) any {
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
	procs := make(map[string]*ast.ProcedureDecl, len(prog.Procedures))
	for _, proc := range prog.Procedures {
		procs[proc.Name.Literal] = proc
		procs[strings.ToLower(proc.Name.Literal)] = proc
	}
	interp := interpreter.New()
	interp.LookupSiblingProc = func(_ string, name string) *ast.ProcedureDecl {
		return procs[strings.ToLower(name)]
	}
	obj := runtime.NewObject("Test", metadata.KindDocument)
	var result any
	if err := interp.RunWithResult(prog.Procedures[0], obj, &result); err != nil {
		t.Fatalf("run: %v", err)
	}
	return result
}

// ─── Массив ───────────────────────────────────────────────────────────────────

func TestArray_AddCountIndex(t *testing.T) {
	src := `Функция Тест()
  а = Новый Массив;
  а.Добавить("x");
  а.Добавить("y");
  Возврат а[0];
КонецФункции`

	result := evalFunc(t, src)
	if result != "x" {
		t.Fatalf("expected x, got %v", result)
	}
}

func TestArray_Literal(t *testing.T) {
	src := `Функция Тест()
  а = ["x", "y", 3];
  Возврат а[1];
КонецФункции`

	result := evalFunc(t, src)
	if result != "y" {
		t.Fatalf("expected y, got %v", result)
	}
}

func TestArray_Count(t *testing.T) {
	src := `Функция Тест()
  а = Новый Массив;
  а.Добавить(1);
  а.Добавить(2);
  а.Добавить(3);
  Возврат а.Количество();
КонецФункции`

	result := evalFunc(t, src)
	if !numEq(result, 3) {
		t.Fatalf("expected 3, got %v", result)
	}
}

func TestArray_ForEach(t *testing.T) {
	src := `Функция Тест()
  а = Новый Массив;
  а.Добавить(10);
  а.Добавить(20);
  а.Добавить(30);
  сумма = 0;
  Для Каждого Эл Из а Цикл
    сумма = сумма + Эл;
  КонецЦикла;
  Возврат сумма;
КонецФункции`

	result := evalFunc(t, src)
	if !numEq(result, 60) {
		t.Fatalf("expected 60, got %v", result)
	}
}

func TestArray_RepeatedForEachDoesNotConsume(t *testing.T) {
	src := `Функция Тест()
  а = Новый Массив;
  а.Добавить(10);
  а.Добавить(20);
  сумма1 = 0;
  Для Каждого Эл Из а Цикл
    сумма1 = сумма1 + Эл;
  КонецЦикла;
  сумма2 = 0;
  Для Каждого Эл Из а Цикл
    сумма2 = сумма2 + Эл;
  КонецЦикла;
  Возврат Строка(а.Количество()) + ":" + Строка(сумма1) + ":" + Строка(сумма2);
КонецФункции`

	result := evalFunc(t, src)
	if result != "2:30:30" {
		t.Fatalf("expected 2:30:30, got %v", result)
	}
}

func TestArray_RepeatedLookupInHelperDoesNotConsume(t *testing.T) {
	src := `Функция Тест()
  Факт = Новый Массив;
  Факт.Добавить("20.1|10.1");
  Факт.Добавить("28.1|20.1");
  N1 = ФактПоКлючу(Факт, "20.1|10.1");
  N2 = ФактПоКлючу(Факт, "28.1|20.1");
  Возврат Строка(Факт.Количество()) + ":" + Строка(N1) + ":" + Строка(N2);
КонецФункции

Функция ФактПоКлючу(Факт, Ключ)
  N = Факт.Количество();
  Для k = 0 По N - 1 Цикл
    Ф = Факт.Получить(k);
    Если Ф = Ключ Тогда
      Возврат N;
    КонецЕсли;
  КонецЦикла;
  Возврат N;
КонецФункции`

	result := evalProgramFunc(t, src)
	if result != "2:2:2" {
		t.Fatalf("expected 2:2:2, got %v", result)
	}
}

func TestArray_IndexAssign(t *testing.T) {
	src := `Функция Тест()
  а = Новый Массив;
  а.Добавить(0);
  а[0] = 42;
  Возврат а[0];
КонецФункции`

	result := evalFunc(t, src)
	if !numEq(result, 42) {
		t.Fatalf("expected 42, got %v", result)
	}
}

// ─── Соответствие ─────────────────────────────────────────────────────────────

func TestMap_InsertGet(t *testing.T) {
	src := `Функция Тест()
  м = Новый Соответствие;
  м.Вставить("USD", 90);
  Возврат м.Получить("USD");
КонецФункции`

	result := evalFunc(t, src)
	if !numEq(result, 90) {
		t.Fatalf("expected 90, got %v", result)
	}
}

func TestMap_ForEach_KeyValue(t *testing.T) {
	src := `Функция Тест()
  м = Новый Соответствие;
  м.Вставить("a", 1);
  м.Вставить("b", 2);
  сумма = 0;
  Для Каждого КЗ Из м Цикл
    сумма = сумма + КЗ.Значение;
  КонецЦикла;
  Возврат сумма;
КонецФункции`

	result := evalFunc(t, src)
	if !numEq(result, 3) {
		t.Fatalf("expected 3, got %v", result)
	}
}

func TestMap_RepeatedForEachDoesNotConsume(t *testing.T) {
	src := `Функция Тест()
  м = Новый Соответствие;
  м.Вставить("a", 1);
  м.Вставить("b", 2);
  сумма1 = 0;
  Для Каждого КЗ Из м Цикл
    сумма1 = сумма1 + КЗ.Значение;
  КонецЦикла;
  сумма2 = 0;
  Для Каждого КЗ Из м Цикл
    сумма2 = сумма2 + КЗ.Значение;
  КонецЦикла;
  Возврат Строка(м.Количество()) + ":" + Строка(сумма1) + ":" + Строка(сумма2);
КонецФункции`

	result := evalFunc(t, src)
	if result != "2:3:3" {
		t.Fatalf("expected 2:3:3, got %v", result)
	}
}

func TestMap_RepeatedGetInHelperDoesNotConsume(t *testing.T) {
	src := `Функция Тест()
  Данные = Новый Соответствие;
  Данные.Вставить("a", 10);
  Данные.Вставить("b", 20);
  R1 = ЗначениеПоКлючу(Данные, "a");
  R2 = ЗначениеПоКлючу(Данные, "b");
  Возврат Строка(Данные.Количество()) + ":" + Строка(R1) + ":" + Строка(R2);
КонецФункции

Функция ЗначениеПоКлючу(Данные, Ключ)
  Возврат Данные.Получить(Ключ);
КонецФункции`

	result := evalProgramFunc(t, src)
	if result != "2:10:20" {
		t.Fatalf("expected 2:10:20, got %v", result)
	}
}

// ─── Структура ────────────────────────────────────────────────────────────────

func TestStruct_DotAccess(t *testing.T) {
	src := `Функция Тест()
  с = Новый Структура("Имя, Возраст", "Иван", 30);
  Возврат с.Имя;
КонецФункции`

	result := evalFunc(t, src)
	if result != "Иван" {
		t.Fatalf("expected Иван, got %v", result)
	}
}

func TestStruct_Insert(t *testing.T) {
	src := `Функция Тест()
  с = Новый Структура;
  с.Вставить("Город", "Москва");
  Возврат с.Свойство("Город");
КонецФункции`

	result := evalFunc(t, src)
	if result != "Москва" {
		t.Fatalf("expected Москва, got %v", result)
	}
}

// ─── Логика: И / ИЛИ / НЕ ────────────────────────────────────────────────────

func TestLogic_And(t *testing.T) {
	src := `Функция Тест()
  Если Истина И Ложь Тогда
    Возврат "да";
  Иначе
    Возврат "нет";
  КонецЕсли;
КонецФункции`

	result := evalFunc(t, src)
	if result != "нет" {
		t.Fatalf("expected нет, got %v", result)
	}
}

func TestLogic_Or(t *testing.T) {
	src := `Функция Тест()
  Если Ложь ИЛИ Истина Тогда
    Возврат "да";
  Иначе
    Возврат "нет";
  КонецЕсли;
КонецФункции`

	result := evalFunc(t, src)
	if result != "да" {
		t.Fatalf("expected да, got %v", result)
	}
}

func TestLogic_Not(t *testing.T) {
	src := `Функция Тест()
  Если НЕ Ложь Тогда
    Возврат "да";
  Иначе
    Возврат "нет";
  КонецЕсли;
КонецФункции`

	result := evalFunc(t, src)
	if result != "да" {
		t.Fatalf("expected да, got %v", result)
	}
}

// Регресс: Сред без 3-го аргумента (длины) должен возвращать остаток строки
// до конца, как в 1С. Раньше возвращал пустую строку (length=0 → end=start).
func TestMid_NoLengthReturnsRest(t *testing.T) {
	cases := map[string]struct{ src, want string }{
		"кириллица_до_конца": {`Функция Тест()
  Возврат Сред("РасчСчет=40702810662130001176", 10);
КонецФункции`, "40702810662130001176"},
		"ascii_до_конца": {`Функция Тест()
  Возврат Сред("abcdef", 3);
КонецФункции`, "cdef"},
		"с_длиной": {`Функция Тест()
  Возврат Сред("abcdef", 3, 2);
КонецФункции`, "cd"},
	}
	for name, c := range cases {
		if got := evalFunc(t, c.src); got != c.want {
			t.Errorf("%s: Сред → %q, ожидалось %q", name, got, c.want)
		}
	}
}

// Регресс: присваивание поля структуры через точку (с.Поле = X) должно
// регистрировать ключ — иначе Количество()/Для Каждого/WriteJSON его не видят.
func TestStruct_DotAssignRegistersKey(t *testing.T) {
	src := `Функция Тест()
  с = Новый Структура;
  с.Всего = 20;
  с.Имя = "тест";
  Возврат с.Количество();
КонецФункции`
	if got := evalFunc(t, src); got != 2.0 {
		t.Fatalf("Количество() → %v, ожидалось 2", got)
	}
}
