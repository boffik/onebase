package cli

import "os"

// noGUI отключает модальные окна с ошибками. Ставится флагом --no-gui.
var noGUI bool

// guiErrorsDisabled сообщает, что модальные окна показывать нельзя: либо явно
// задан флаг --no-gui, либо переменная окружения ONEBASE_NO_GUI. Нужно для
// скриптов и CI, где некому закрыть всплывающее окно (иначе процесс зависает).
func guiErrorsDisabled() bool {
	return noGUI || os.Getenv("ONEBASE_NO_GUI") != ""
}
