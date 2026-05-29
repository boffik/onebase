package interpreter

import "testing"

// numEq сравнивает числовой DSL-результат (теперь decimal.Decimal) с ожидаемым
// значением. Для целочисленных ожиданий InexactFloat64 точен.
func numEq(got any, want float64) bool {
	f, ok := toFloat(got)
	return ok && f == want
}

// TestWhile_Basic — классический счётчик через Пока ... Цикл.
func TestWhile_Basic(t *testing.T) {
	result := evalBreakFunc(t, `Функция Тест()
  Сумма = 0;
  i = 1;
  Пока i <= 5 Цикл
    Сумма = Сумма + i;
    i = i + 1;
  КонецЦикла;
  Возврат Сумма;
КонецФункции`)
	if !numEq(result, 15) {
		t.Errorf("expected 15, got %v", result)
	}
}

// TestWhile_Break — Прервать выходит из цикла Пока.
func TestWhile_Break(t *testing.T) {
	result := evalBreakFunc(t, `Функция Тест()
  Сумма = 0;
  i = 0;
  Пока Истина Цикл
    i = i + 1;
    Если i > 3 Тогда
      Прервать;
    КонецЕсли;
    Сумма = Сумма + i;
  КонецЦикла;
  Возврат Сумма;
КонецФункции`)
	if !numEq(result, 6) {
		t.Errorf("expected 6, got %v", result)
	}
}

// TestWhile_Continue — Продолжить переходит к следующей итерации Пока.
func TestWhile_Continue(t *testing.T) {
	result := evalBreakFunc(t, `Функция Тест()
  Сумма = 0;
  i = 0;
  Пока i < 5 Цикл
    i = i + 1;
    Если i = 3 Тогда
      Продолжить;
    КонецЕсли;
    Сумма = Сумма + i;
  КонецЦикла;
  Возврат Сумма;
КонецФункции`)
	if !numEq(result, 12) {
		t.Errorf("expected 12, got %v", result)
	}
}

// TestWhile_ConditionFalse — тело не выполняется, если условие ложно сразу.
func TestWhile_ConditionFalse(t *testing.T) {
	result := evalBreakFunc(t, `Функция Тест()
  Счётчик = 0;
  Пока Ложь Цикл
    Счётчик = Счётчик + 1;
  КонецЦикла;
  Возврат Счётчик;
КонецФункции`)
	if !numEq(result, 0) {
		t.Errorf("expected 0, got %v", result)
	}
}

// TestWhile_EnglishKeyword — англоязычное ключевое слово While.
func TestWhile_EnglishKeyword(t *testing.T) {
	result := evalBreakFunc(t, `Функция Тест()
  n = 0;
  While n < 10 Цикл
    n = n + 2;
  КонецЦикла;
  Возврат n;
КонецФункции`)
	if !numEq(result, 10) {
		t.Errorf("expected 10, got %v", result)
	}
}

// TestRef_Наименование — ссылка.Наименование возвращает имя объекта, а не nil.
func TestRef_Наименование(t *testing.T) {
	ref := &Ref{UUID: "11111111-1111-1111-1111-111111111111", Name: "Главный склад"}
	if got := ref.Get("Наименование"); got != "Главный склад" {
		t.Errorf("Наименование: expected %q, got %v", "Главный склад", got)
	}
	if got := ref.Get("Имя"); got != "Главный склад" {
		t.Errorf("Имя: expected %q, got %v", "Главный склад", got)
	}
	if got := ref.Get("УникальныйИдентификатор"); got != ref.UUID {
		t.Errorf("УникальныйИдентификатор: expected %q, got %v", ref.UUID, got)
	}
	if got := ref.Get("Ссылка"); got != ref {
		t.Errorf("Ссылка: expected self, got %v", got)
	}
	if got := ref.Get("НетТакогоПоля"); got != nil {
		t.Errorf("unknown field: expected nil, got %v", got)
	}
}
