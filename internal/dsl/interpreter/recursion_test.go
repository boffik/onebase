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
)

// Бесконечная рекурсия должна оборваться осмысленной ошибкой, а не падением
// процесса от переполнения стека горутины (аудит, п.5). Порог понижаем через
// поле Interpreter, чтобы страж срабатывал быстро и детерминированно (достижение
// штатной глубины 1000 само по себе медленно из-за O(глубина) поиска имён в env).
func TestRecursion_DepthGuard(t *testing.T) {
	src := `Функция Рекурс(Н)
  Возврат Рекурс(Н + 1);
КонецФункции`
	prog, err := parser.New(lexer.New(src, "test.os")).ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	interp := interpreter.New()
	interp.MaxRecursionDepth = 50
	// Самовызов по имени резолвится через LookupProc.
	interp.LookupProc = func(name string) *ast.ProcedureDecl {
		for _, p := range prog.Procedures {
			if strings.EqualFold(p.Name.Literal, name) {
				return p
			}
		}
		return nil
	}
	obj := runtime.NewObject("Test", metadata.KindDocument)
	var result any
	runErr := interp.RunWithResult(prog.Procedures[0], obj, &result, map[string]any{"Н": 0})
	if runErr == nil {
		t.Fatal("ожидалась ошибка превышения глубины рекурсии, получено nil")
	}
	if !strings.Contains(runErr.Error(), "глубина рекурсии") {
		t.Fatalf("неожиданная ошибка: %v", runErr)
	}
}
