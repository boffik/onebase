//go:build windows

package cli

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// fileTypeUnknown — GetFileType возвращает это значение, когда хэндл не привязан
// к консоли, пайпу или файлу (например, нет консоли при запуске двойным кликом).
const fileTypeUnknown = 0x0000 // FILE_TYPE_UNKNOWN

// showError сообщает об ошибке запуска. Текст всегда пишется в stderr. Модальное
// окно MessageBox показывается ТОЛЬКО когда писать ошибку реально некуда —
// то есть stderr недоступен (запуск двойным кликом по onebase.exe без консоли).
// При запуске из консоли, скрипта или CI (stderr ведёт в консоль/пайп/файл)
// окно не показывается — иначе фоновый процесс зависал бы на модалке, которую
// некому закрыть. Флаг --no-gui / ONEBASE_NO_GUI отключает окно принудительно.
func showError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	if guiErrorsDisabled() || stderrIsUsable() {
		return
	}
	dll := syscall.NewLazyDLL("user32.dll")
	proc := dll.NewProc("MessageBoxW")
	title, _ := syscall.UTF16PtrFromString("onebase — Ошибка запуска")
	text, _ := syscall.UTF16PtrFromString(msg)
	proc.Call(0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		0x10) // MB_ICONERROR
}

// stderrIsUsable возвращает true, если stderr ведёт в консоль, пайп или файл —
// то есть текст ошибки действительно куда-то попадёт. Для недоступного хэндла
// (нет консоли) GetFileType вернёт FILE_TYPE_UNKNOWN.
func stderrIsUsable() bool {
	h := syscall.Handle(os.Stderr.Fd())
	if h == 0 || h == syscall.InvalidHandle {
		return false
	}
	dll := syscall.NewLazyDLL("kernel32.dll")
	proc := dll.NewProc("GetFileType")
	// GetFileType не выставляет осмысленный lastError для FILE_TYPE_UNKNOWN,
	// поэтому решение принимаем по возвращаемому значению.
	r, _, _ := proc.Call(uintptr(h))
	return uint32(r) != fileTypeUnknown
}
