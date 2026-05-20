package cli

import (
	"strings"
	"testing"
)

// защита от случайной активации demo на проде.
func TestCheckDemoEnv_Allowed(t *testing.T) {
	if err := checkDemoEnv("demo"); err != nil {
		t.Errorf("ONEBASE_ENV=demo должен пропускать, получили: %v", err)
	}
}

func TestCheckDemoEnv_Empty(t *testing.T) {
	err := checkDemoEnv("")
	if err == nil {
		t.Fatal("пустой ONEBASE_ENV должен блокировать запуск")
	}
	if !strings.Contains(err.Error(), "ONEBASE_ENV") {
		t.Errorf("ошибка должна упоминать ONEBASE_ENV, получили: %v", err)
	}
}

func TestCheckDemoEnv_Prod(t *testing.T) {
	err := checkDemoEnv("production")
	if err == nil {
		t.Fatal("ONEBASE_ENV=production должен блокировать запуск с demo.enabled=true")
	}
}

func TestCheckDemoEnv_TypoNotAllowed(t *testing.T) {
	for _, val := range []string{"Demo", "DEMO", "demo ", " demo", "test"} {
		if err := checkDemoEnv(val); err == nil {
			t.Errorf("ONEBASE_ENV=%q не должен совпадать с %q, ожидалась ошибка", val, "demo")
		}
	}
}
