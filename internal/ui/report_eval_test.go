package ui

import (
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/report/compose"
)

func TestInterpEvaluator(t *testing.T) {
	ev := newInterpEvaluator(interpreter.New())
	ok, err := ev.EvalBool("Сумма < 0", compose.Row{"Сумма": "-45"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ожидали true для Сумма<0")
	}
	ok, _ = ev.EvalBool("Сумма < 0", compose.Row{"Сумма": "10"})
	if ok {
		t.Fatal("ожидали false для Сумма>=0")
	}
}
