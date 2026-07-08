package interpreter_test

import "testing"

// План 42: арифметика на decimal должна быть точной — без float-погрешности.
// Проверяем как само значение, так и его строковое представление (Строка),
// потому что именно через Строка() пользователь видел «0.10500000000000001».

func TestDecimal_PriceRecalc(t *testing.T) {
	// Кейс из исходного комментария: 0.1 → 0.1 * 1.05.
	src := `Функция Тест()
  Возврат Строка(0.1 * (1 + 5 / 100));
КонецФункции`
	if got := evalFunc(t, src); got != "0.105" {
		t.Fatalf("0.1 * 1.05 → %q, ожидалось \"0.105\"", got)
	}
}

func TestDecimal_ClassicFloatTraps(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"0.1 + 0.2", "0.3"},
		{"0.1 * 1.05", "0.105"},
		{"1.1 - 1.0", "0.1"},
		{"1.005 * 100", "100.5"},
		{"0 - 0.3", "-0.3"},
	}
	for _, c := range cases {
		src := "Функция Тест()\n  Возврат Строка(" + c.expr + ");\nКонецФункции"
		if got := evalFunc(t, src); got != c.want {
			t.Errorf("%s → %q, ожидалось %q", c.expr, got, c.want)
		}
	}
}

func TestDecimal_SumOfHundredths(t *testing.T) {
	// 1000 × 0.01 = 10.00 — классическое накопление float-ошибки.
	src := `Функция Тест()
  Сумма = 0;
  Для i = 1 По 1000 Цикл
    Сумма = Сумма + 0.01;
  КонецЦикла;
  Возврат Строка(Сумма);
КонецФункции`
	if got := evalFunc(t, src); got != "10" {
		t.Fatalf("1000×0.01 → %q, ожидалось \"10\"", got)
	}
}

func TestDecimal_DivisionPrecision(t *testing.T) {
	// Деление с бесконечной дробью обрезается до DivisionPrecision (16),
	// без float-мусора. Округление до результата — задача Окр.
	src := `Функция Тест()
  Возврат Строка(10 / 3);
КонецФункции`
	want := "3.3333333333333333" // 16 знаков
	if got := evalFunc(t, src); got != want {
		t.Fatalf("10/3 → %q, ожидалось %q", got, want)
	}
}

func TestDecimal_RoundModes(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"Окр(2.5, 0)", "3"},      // half away from zero (как 1С)
		{"Окр(2.5, 0, 1)", "2"},   // банковское (half to even)
		{"Окр(3.5, 0, 1)", "4"},   // банковское
		{"Окр(1.005, 2)", "1.01"}, // точное округление, без float-ловушки
		{"Окр(2.555, 2)", "2.56"}, //
		{"Окр(-2.5, 0)", "-3"},    // от нуля
	}
	for _, c := range cases {
		src := "Функция Тест()\n  Возврат Строка(" + c.expr + ");\nКонецФункции"
		if got := evalFunc(t, src); got != c.want {
			t.Errorf("%s → %q, ожидалось %q", c.expr, got, c.want)
		}
	}
}

func TestDecimal_NumberParsesComma(t *testing.T) {
	// Число("1,5") — запятая как десятичный разделитель.
	src := `Функция Тест()
  Возврат Строка(Число("1,5") + Число("2,5"));
КонецФункции`
	if got := evalFunc(t, src); got != "4" {
		t.Fatalf("Число(\"1,5\")+Число(\"2,5\") → %q, ожидалось \"4\"", got)
	}
}

func TestToString_StringArgumentIsIdentity(t *testing.T) {
	src := `Функция Тест()
  Возврат Строка("40802810000000000001");
КонецФункции`
	if got := evalFunc(t, src); got != "40802810000000000001" {
		t.Fatalf("Строка(строка) → %q, ожидалась исходная строка", got)
	}
}

func TestDecimal_Comparison(t *testing.T) {
	// 0.1+0.2 == 0.3 — в float это было бы Ложь.
	src := `Функция Тест()
  Если 0.1 + 0.2 = 0.3 Тогда
    Возврат "равно";
  Иначе
    Возврат "не равно";
  КонецЕсли;
КонецФункции`
	if got := evalFunc(t, src); got != "равно" {
		t.Fatalf("0.1+0.2 = 0.3 → %q, ожидалось \"равно\"", got)
	}
}
