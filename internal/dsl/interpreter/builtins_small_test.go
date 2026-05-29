package interpreter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArray_FindSetBound(t *testing.T) {
	a := &Array{}
	a.CallMethod("добавить", []any{"x"})
	a.CallMethod("добавить", []any{float64(10)})

	if got := a.CallMethod("найти", []any{float64(10)}); got != float64(1) {
		t.Errorf("Найти(10): ожидался индекс 1, got %v", got)
	}
	if got := a.CallMethod("найти", []any{"нет"}); got != nil {
		t.Errorf("Найти несуществующего → Неопределено, got %v", got)
	}
	a.CallMethod("установить", []any{float64(0), "y"})
	if a.Index(0) != "y" {
		t.Errorf("Установить(0,y): got %v", a.Index(0))
	}
	if got := a.CallMethod("вграница", nil); got != float64(1) {
		t.Errorf("ВГраница: ожидалось 1, got %v", got)
	}
	if got := (&Array{}).CallMethod("вграница", nil); got != float64(-1) {
		t.Errorf("ВГраница пустого: ожидалось -1, got %v", got)
	}
}

func TestFileOps(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	callB(t, "создатькаталог", sub)
	if st, err := os.Stat(sub); err != nil || !st.IsDir() {
		t.Fatalf("СоздатьКаталог не создал каталог: %v", err)
	}

	src := filepath.Join(sub, "a.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(sub, "b.txt")
	callB(t, "копироватьфайл", src, dst)
	if b, err := os.ReadFile(dst); err != nil || string(b) != "hi" {
		t.Fatalf("КопироватьФайл: %q err=%v", b, err)
	}

	arr := callB(t, "найтифайлы", sub, "*.txt").(*Array)
	if len(arr.items) != 2 {
		t.Errorf("НайтиФайлы: ожидалось 2 файла, got %d", len(arr.items))
	}

	moved := filepath.Join(sub, "c.txt")
	callB(t, "переместитьфайл", dst, moved)
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("ПереместитьФайл: исходник должен исчезнуть")
	}
	callB(t, "удалитьфайлы", moved)
	if _, err := os.Stat(moved); !os.IsNotExist(err) {
		t.Error("УдалитьФайлы: файл должен быть удалён")
	}
}
