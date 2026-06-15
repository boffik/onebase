package launcher

import (
	"strings"
	"testing"
)

func TestQueryHintSystem(t *testing.T) {
	s := queryHintSystem("Справочники:\n  Клиент: Наименование")
	for _, sub := range []string{"ВЫБРАТЬ", "Остатки", "Клиент", "Конфигурация базы"} {
		if !strings.Contains(s, sub) {
			t.Errorf("queryHintSystem не содержит %q:\n%s", sub, s)
		}
	}
	if got := queryHintSystem(""); strings.Contains(got, "Конфигурация базы") {
		t.Error("пустой schema не должен добавлять секцию конфигурации")
	}
}
