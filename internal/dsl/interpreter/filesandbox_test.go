package interpreter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSafePath_Disabled(t *testing.T) {
	SetFileSandbox("")
	p, err := resolveSafePath("/etc/passwd")
	if err != nil || p != "/etc/passwd" {
		t.Errorf("выключенный sandbox должен пропускать путь: p=%q err=%v", p, err)
	}
}

func TestResolveSafePath_WithinAndEscape(t *testing.T) {
	root := t.TempDir()
	SetFileSandbox(root)
	defer SetFileSandbox("")

	if _, err := resolveSafePath("sub/a.txt"); err != nil {
		t.Errorf("путь внутри корня должен быть разрешён: %v", err)
	}
	if _, err := resolveSafePath("../secret.txt"); err == nil {
		t.Error("выход через .. должен быть запрещён")
	}
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	if _, err := resolveSafePath(outside); err == nil {
		t.Error("абсолютный путь вне корня должен быть запрещён")
	}
	inside := filepath.Join(root, "x.txt")
	if got, err := resolveSafePath(inside); err != nil || got != filepath.Clean(inside) {
		t.Errorf("абсолютный путь внутри корня: got=%q err=%v", got, err)
	}
}

// Файловые builtins должны соблюдать sandbox: внутри корня — ок,
// попытка выйти за пределы — прерывание (panic userError).
func TestFileBuiltins_RespectSandbox(t *testing.T) {
	root := t.TempDir()
	SetFileSandbox(root)
	defer SetFileSandbox("")

	inside := filepath.Join(root, "a.txt")
	if err := os.WriteFile(inside, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	// копирование внутри корня — ок
	callB(t, "копироватьфайл", inside, filepath.Join(root, "b.txt"))

	// копирование за пределы корня — должно прерваться
	outside := filepath.Join(filepath.Dir(root), "evil.txt")
	assertRaises(t, func() {
		_, _ = builtins["копироватьфайл"]([]any{inside, outside}, "", 0)
	})
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Error("файл за пределами sandbox не должен быть создан")
	}

	// чтение системного файла через ЧтениеТекста — тоже за пределами
	reader := &dslTextReader{path: "/etc/passwd"}
	assertRaises(t, func() { reader.CallMethod("открыть", nil) })
}

// assertRaises проверяет, что fn прерывается паникой (userError из sandbox).
func assertRaises(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("ожидалось прерывание (доступ вне sandbox), но его не было")
		}
	}()
	fn()
}
