package cli

import (
	"os"
	"testing"
)

// guiErrorsDisabled решает, можно ли показывать модальное окно с ошибкой.
// Для скриптов/CI окно должно подавляться флагом --no-gui или ONEBASE_NO_GUI.
func TestGUIErrorsDisabled(t *testing.T) {
	old, had := os.LookupEnv("ONEBASE_NO_GUI")
	t.Cleanup(func() {
		noGUI = false
		if had {
			os.Setenv("ONEBASE_NO_GUI", old)
		} else {
			os.Unsetenv("ONEBASE_NO_GUI")
		}
	})

	// По умолчанию модалки разрешены.
	noGUI = false
	os.Unsetenv("ONEBASE_NO_GUI")
	if guiErrorsDisabled() {
		t.Error("по умолчанию модальные окна должны быть разрешены")
	}

	// Флаг --no-gui отключает.
	noGUI = true
	if !guiErrorsDisabled() {
		t.Error("флаг --no-gui должен отключать модальные окна")
	}

	// Переменная окружения отключает.
	noGUI = false
	os.Setenv("ONEBASE_NO_GUI", "1")
	if !guiErrorsDisabled() {
		t.Error("ONEBASE_NO_GUI=1 должен отключать модальные окна")
	}

	// Пустое значение переменной не считается включённым.
	os.Setenv("ONEBASE_NO_GUI", "")
	if guiErrorsDisabled() {
		t.Error("пустой ONEBASE_NO_GUI не должен отключать окна")
	}
}
