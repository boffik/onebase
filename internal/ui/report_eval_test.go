package ui

import (
	"testing"

	"github.com/shopspring/decimal"

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

func TestInterpEvaluatorEvalNum(t *testing.T) {
	ev := newInterpEvaluator(interpreter.New())

	// Кейс 1: деление — Выручка / 2 при Выручка=100 → 50.
	row := compose.Row{"Выручка": decimal.NewFromInt(100)}
	d, ok, err := ev.EvalNum("Выручка / 2", row)
	if err != nil {
		t.Fatalf("EvalNum вернул ошибку: %v", err)
	}
	if !ok {
		t.Fatal("EvalNum: ok=false, ожидали ok=true")
	}
	if !d.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("EvalNum: ожидали 50, получили %v", d)
	}

	// Кейс 2: умножение двух decimal-переменных — А * Б при А=7, Б=3 → 21.
	row2 := compose.Row{
		"А": decimal.NewFromInt(7),
		"Б": decimal.NewFromInt(3),
	}
	d2, ok2, err2 := ev.EvalNum("А * Б", row2)
	if err2 != nil {
		t.Fatalf("EvalNum (А*Б) вернул ошибку: %v", err2)
	}
	if !ok2 {
		t.Fatal("EvalNum (А*Б): ok=false, ожидали ok=true")
	}
	if !d2.Equal(decimal.NewFromInt(21)) {
		t.Fatalf("EvalNum (А*Б): ожидали 21, получили %v", d2)
	}

	// Кейс 3: строковое значение приводится к decimal через toDecimal —
	// ExportToDecimal умеет читать строки, поэтому строка "10" тоже должна работать.
	row3 := compose.Row{"Цена": "10", "Кол": decimal.NewFromInt(5)}
	d3, ok3, err3 := ev.EvalNum("Цена * Кол", row3)
	if err3 != nil {
		t.Fatalf("EvalNum (Цена*Кол строка) вернул ошибку: %v", err3)
	}
	if !ok3 {
		t.Fatal("EvalNum (Цена*Кол строка): ok=false, ожидали ok=true")
	}
	if !d3.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("EvalNum (Цена*Кол строка): ожидали 50, получили %v", d3)
	}
}
