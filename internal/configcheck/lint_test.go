package configcheck

import (
	"path/filepath"
	"testing"
)

func TestRunFullWithLintWarnings(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, filepath.Join(dir, "catalogs", "клиент.yaml"), `name: Клиент
unknown_top_key: true
fields:
  - name: Наименование
    type: string
`)
	mkFile(t, filepath.Join(dir, "documents", "заказ.yaml"), `name: Заказ
fields:
  - name: Номер
    type: string
`)
	mkFile(t, filepath.Join(dir, "processors", "мусор.yaml"), `name: Мусор
params: []
`)
	mkFile(t, filepath.Join(dir, "src", "мусор.proc.os"), `Процедура Выполнить() Экспорт
  Перем Лишняя, Нужная;
  Нужная = 1;
  Сообщить(Нужная);
КонецПроцедуры

Процедура Мертвая()
КонецПроцедуры
`)
	mkFile(t, filepath.Join(dir, "roles", "оператор.yaml"), `name: Оператор
permissions:
  catalogs:
    Клиент: [read]
  processors: {}
`)

	plain := RunFull(dir)
	if !plain.OK {
		t.Fatalf("plain check should be OK: %+v", plain.Issues)
	}
	for _, w := range plain.Warnings {
		if w.Code == "metadata.unvalidated-key" || w.Code == "dsl.unused-var" ||
			w.Code == "dsl.dead-procedure" || w.Code == "rbac.object-without-role" {
			t.Fatalf("plain RunFull unexpectedly returned lint warning: %+v", w)
		}
	}

	lint := RunFullWithOptions(dir, Options{Lint: true})
	if !lint.OK {
		t.Fatalf("lint check should keep OK=true for warnings: %+v", lint.Issues)
	}
	want := map[string]bool{
		"metadata.unvalidated-key": false,
		"dsl.unused-var":           false,
		"dsl.dead-procedure":       false,
		"rbac.object-without-role": false,
	}
	for _, w := range lint.Warnings {
		if _, ok := want[w.Code]; ok {
			want[w.Code] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Fatalf("lint warning %s not found; got %+v", code, lint.Warnings)
		}
	}
}
