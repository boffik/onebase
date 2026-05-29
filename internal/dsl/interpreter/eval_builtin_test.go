package interpreter_test

import "testing"

func TestEval_Arithmetic(t *testing.T) {
	src := `Функция Тест()
  Возврат Вычислить("2 + 3 * 4");
КонецФункции`
	if got := runFunc(t, src); got != float64(14) {
		t.Fatalf("Вычислить арифметики: ожидалось 14, got %v", got)
	}
}

// Вычислить видит локальные переменные текущего окружения.
func TestEval_LocalVar(t *testing.T) {
	src := `Функция Тест()
  х = 5;
  Возврат Вычислить("х * 2");
КонецФункции`
	if got := runFunc(t, src); got != float64(10) {
		t.Fatalf("Вычислить с локальной переменной: ожидалось 10, got %v", got)
	}
}
