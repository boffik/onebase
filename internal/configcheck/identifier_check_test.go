package configcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFullReportsInvalidIdentifierWithFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "catalogs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalogs", "клиенты.yaml"), []byte(`name: Клиенты
fields:
  - name: Приход  От Клиента
    type: number
`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := RunFull(dir)
	if res.OK {
		t.Fatalf("RunFull OK=true, ожидалась ошибка")
	}
	for _, is := range res.Issues {
		if is.File == "catalogs/клиенты.yaml" &&
			strings.Contains(is.Message, "Приход  От Клиента") &&
			strings.Contains(is.Message, "без пробелов") {
			if is.Line != 3 {
				t.Fatalf("line = %d, want 3: %+v", is.Line, is)
			}
			return
		}
	}
	t.Fatalf("нет ошибки по конкретному YAML-файлу: %+v", res.Issues)
}
