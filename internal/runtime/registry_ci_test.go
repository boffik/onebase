package runtime

import (
	"testing"

	"github.com/boffik/onebase/internal/dsl/ast"
	"github.com/boffik/onebase/internal/dsl/lexer"
	"github.com/boffik/onebase/internal/dsl/parser"
	"github.com/boffik/onebase/internal/processor"
)

// #13: внешний объект с именем в другом регистре, чем у конфигурационного, не
// должен дублироваться в списке (Processors/Reports) — busy-check теперь
// регистронезависимый, как GetProcessor/GetReport.
func TestProcessors_CaseInsensitiveBusyCheck(t *testing.T) {
	r := NewRegistry()
	r.mu.Lock()
	r.processors["Продажи"] = &processor.Processor{Name: "Продажи"}
	r.mu.Unlock()

	prog, err := parser.New(lexer.New("Процедура Выполнить()\nКонецПроцедуры\n", "x.proc.os")).ParseProgram()
	if err != nil {
		t.Fatal(err)
	}
	// Внешняя «продажи» (нижний регистр) совпадает с конфигурационной «Продажи».
	r.SetExternalProcessors(
		[]*processor.Processor{{Name: "продажи"}},
		map[string]*ast.Program{"продажи": prog},
	)

	if got := len(r.Processors()); got != 1 {
		names := make([]string, 0)
		for _, p := range r.Processors() {
			names = append(names, p.Name)
		}
		t.Errorf("внешняя «продажи» при конфигурационной «Продажи» не должна дублироваться: got %d (%v)", got, names)
	}
}
