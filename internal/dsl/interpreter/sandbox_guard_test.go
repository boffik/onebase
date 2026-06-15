package interpreter

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deny-guard блокирует файловую операцию до реального доступа к ФС.
func TestNewFileFunctions_GuardBlocks(t *testing.T) {
	deny := FileGuard(func() error { return errors.New("файлы запрещены") })
	m := NewFileFunctions(deny)
	msg := callBuiltinExpectPanic(t, m["копироватьфайл"], []any{"a.txt", "b.txt"})
	if !strings.Contains(msg, "файлы запрещены") {
		t.Errorf("ожидалось сообщение guard'а, получено %q", msg)
	}
}

// nil-guard не блокирует: копирование реального файла проходит.
func TestNewFileFunctions_NilGuardAllows(t *testing.T) {
	SetFileSandbox("")
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "b.txt")
	fn, ok := NewFileFunctions(nil)["копироватьфайл"].(BuiltinFunc)
	if !ok {
		t.Fatal("копироватьфайл должна быть BuiltinFunc")
	}
	if _, err := fn([]any{src, dst}, "", 0); err != nil {
		t.Fatalf("nil-guard не должен блокировать: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("файл должен быть скопирован: %v", err)
	}
}
