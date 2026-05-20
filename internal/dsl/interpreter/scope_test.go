package interpreter

import (
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
)

// runScopeFunc parses a function and returns its result. Stays in the
// `interpreter` package so we can access the unexported Run helpers.
func runScopeFunc(t *testing.T, code string) any {
	t.Helper()
	l := lexer.New(code, "<test>")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Procedures) == 0 {
		t.Fatal("no procedures parsed")
	}
	i := New()
	this := &MapThis{M: map[string]any{}}
	var result any
	if err := i.RunWithResult(prog.Procedures[0], this, &result); err != nil {
		t.Fatalf("run: %v", err)
	}
	return result
}

// Регрессия для замечания #22: присваивание в ветви Если/Иначе должно
// обновлять переменную внешнего scope, а не создавать локальную.
func TestIfElseScope_ThenAssignmentPropagates(t *testing.T) {
	code := `Функция Тест()
  Действие = "?";
  Если 1 = 1 Тогда
    Действие = "Обновлён";
  Иначе
    Действие = "Создан";
  КонецЕсли;
  Возврат Действие;
КонецФункции`
	if got := runScopeFunc(t, code); got != "Обновлён" {
		t.Errorf("expected \"Обновлён\", got %v", got)
	}
}

func TestIfElseScope_ElseAssignmentPropagates(t *testing.T) {
	code := `Функция Тест()
  Действие = "?";
  Если 1 = 2 Тогда
    Действие = "Обновлён";
  Иначе
    Действие = "Создан";
  КонецЕсли;
  Возврат Действие;
КонецФункции`
	if got := runScopeFunc(t, code); got != "Создан" {
		t.Errorf("expected \"Создан\", got %v", got)
	}
}

// Та же история, но переменная объявлена внутри Для-цикла — тоже частый
// сценарий (Эл из коллекции, действие в зависимости от него).
func TestIfElseScope_InsideForEach(t *testing.T) {
	code := `Функция Тест()
  М = Новый Массив;
  М.Добавить(1);
  Лог = "";
  Для Каждого Э Из М Цикл
    Действие = "?";
    Если Э = 1 Тогда
      Действие = "один";
    Иначе
      Действие = "другой";
    КонецЕсли;
    Лог = Лог + Действие;
  КонецЦикла;
  Возврат Лог;
КонецФункции`
	if got := runScopeFunc(t, code); got != "один" {
		t.Errorf("expected \"один\", got %v", got)
	}
}

// параметры по умолчанию.
func TestDefaultParam_UsedWhenOmitted(t *testing.T) {
	code := `Функция Тест()
  Возврат Сумма(10);
КонецФункции

Функция Сумма(А, Б = 20)
  Возврат А + Б;
КонецФункции`
	// Параллельная процедура должна быть доступна через LookupSiblingProc,
	// но для теста проще — вызов внутри одного файла обходится без него:
	// callUserProc находит helper через i.LookupProc. Здесь нет реестра,
	// но parser кладёт обе процедуры в один Program — а RunWithResult
	// исполняет только первую. Сделаем helper через LookupSiblingProc.
	l := lexer.New(code, "<test>")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Procedures) != 2 {
		t.Fatalf("ожидалось 2 процедуры, получили %d", len(prog.Procedures))
	}
	main, helper := prog.Procedures[0], prog.Procedures[1]
	i := New()
	i.LookupSiblingProc = func(file, name string) *ast.ProcedureDecl {
		if name == "Сумма" || name == "сумма" {
			return helper
		}
		return nil
	}
	var result any
	if err := i.RunWithResult(main, &MapThis{M: map[string]any{}}, &result); err != nil {
		t.Fatalf("run: %v", err)
	}
	if result != float64(30) {
		t.Errorf("expected 30 (10+20 default), got %v", result)
	}
}

func TestDefaultParam_OverriddenByArg(t *testing.T) {
	code := `Функция Тест()
  Возврат Сумма(10, 5);
КонецФункции

Функция Сумма(А, Б = 20)
  Возврат А + Б;
КонецФункции`
	l := lexer.New(code, "<test>")
	p := parser.New(l)
	prog, _ := p.ParseProgram()
	main, helper := prog.Procedures[0], prog.Procedures[1]
	i := New()
	i.LookupSiblingProc = func(_, name string) *ast.ProcedureDecl {
		if name == "Сумма" || name == "сумма" {
			return helper
		}
		return nil
	}
	var result any
	if err := i.RunWithResult(main, &MapThis{M: map[string]any{}}, &result); err != nil {
		t.Fatalf("run: %v", err)
	}
	if result != float64(15) {
		t.Errorf("expected 15, got %v", result)
	}
}

func TestDefaultParam_StringDefault(t *testing.T) {
	code := `Функция Тест()
  Возврат Привет();
КонецФункции

Функция Привет(Имя = "мир")
  Возврат "Hello, " + Имя;
КонецФункции`
	l := lexer.New(code, "<test>")
	p := parser.New(l)
	prog, _ := p.ParseProgram()
	main, helper := prog.Procedures[0], prog.Procedures[1]
	i := New()
	i.LookupSiblingProc = func(_, name string) *ast.ProcedureDecl {
		if name == "Привет" || name == "привет" {
			return helper
		}
		return nil
	}
	var result any
	if err := i.RunWithResult(main, &MapThis{M: map[string]any{}}, &result); err != nil {
		t.Fatalf("run: %v", err)
	}
	if result != "Hello, мир" {
		t.Errorf("expected Hello, мир, got %v", result)
	}
}

// ИначеЕсли тоже должен корректно писать во внешний scope.
func TestIfElseScope_ElseIfAssignmentPropagates(t *testing.T) {
	code := `Функция Тест()
  Действие = "?";
  Если 1 = 2 Тогда
    Действие = "первая";
  ИначеЕсли 1 = 1 Тогда
    Действие = "вторая";
  Иначе
    Действие = "третья";
  КонецЕсли;
  Возврат Действие;
КонецФункции`
	if got := runScopeFunc(t, code); got != "вторая" {
		t.Errorf("expected \"вторая\", got %v", got)
	}
}
