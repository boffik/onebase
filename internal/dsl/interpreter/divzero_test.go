package interpreter_test

import (
	"strings"
	"testing"

	"github.com/boffik/onebase/internal/metadata"
	"github.com/boffik/onebase/internal/runtime"
)

// План 47: деление на ноль должно быть исключением (как в 1С), а не молчаливым nil.
func TestInterpreter_DivisionByZeroRaises(t *testing.T) {
	src := `Procedure Main()
  x = 10 / 0;
EndProcedure`
	err := runProc(t, src, runtime.NewObject("T", metadata.KindCatalog))
	if err == nil {
		t.Fatal("ожидалось исключение при делении на ноль")
	}
	if !strings.Contains(err.Error(), "Деление на ноль") {
		t.Fatalf("ожидалось сообщение 'Деление на ноль', got: %v", err)
	}
}

func TestInterpreter_DivisionByZeroVariable(t *testing.T) {
	src := `Procedure Main()
  d = 0;
  x = 5 / d;
EndProcedure`
	err := runProc(t, src, runtime.NewObject("T", metadata.KindCatalog))
	if err == nil {
		t.Fatal("ожидалось исключение при делении на нулевую переменную")
	}
}

// Нормальное деление по-прежнему работает.
func TestInterpreter_DivisionOk(t *testing.T) {
	src := `Procedure Main()
  x = 10 / 2;
EndProcedure`
	if err := runProc(t, src, runtime.NewObject("T", metadata.KindCatalog)); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
}
