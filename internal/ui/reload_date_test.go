package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
)

// Ссылка.ПолучитьОбъект() у записанного документа должен нести поле-дату как
// настоящую Дату (time.Time), а не строку — иначе арифметика дат (КонецДня,
// Дата + Число) в проведении перезаписанного документа молча ломается.
// Регресс: раньше на SQLite дата грузилась RFC3339-строкой, КонецДня(строка)=nil.
func TestReloadedDocumentDateIsTyped(t *testing.T) {
	dir := t.TempDir()
	mk := func(sub string) string {
		p := filepath.Join(dir, sub)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	docDir := mk("documents")
	procDir := mk("processors")
	srcDir := mk("src")

	if err := os.WriteFile(filepath.Join(docDir, "заказ.yaml"),
		[]byte("name: Заказ\nposting: false\nfields:\n  - name: Дата\n    type: date\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "проба.yaml"),
		[]byte("name: Проба\nparams: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	proc := "Процедура Выполнить()\n" +
		"  Д = Документы.Заказ.Создать();\n" +
		"  Д.Дата = Дата(2026, 2, 20);\n" +
		"  Ссылка = Д.Записать();\n" +
		"  Об = Ссылка.ПолучитьОбъект();\n" +
		"  Кон = КонецДня(Об.Дата);\n" +
		"  Если НЕ ЗначениеЗаполнено(Кон) Тогда\n" +
		"    ВызватьИсключение(\"КонецДня(Об.Дата) пуст — дата загружена строкой\");\n" +
		"  КонецЕсли;\n" +
		"  Сообщить(ТипЗнч(Об.Дата));\n" +
		"КонецПроцедуры\n"
	if err := os.WriteFile(filepath.Join(srcDir, "проба.proc.os"), []byte(proc), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	proj, err := project.Load(dir)
	if err != nil {
		t.Fatalf("project.Load: %v", err)
	}
	defer proj.Close()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "reload.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, proj.Entities); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	messages, runErr, err := RunProcessorOffline(ctx, proj, db, "Проба", nil, nil)
	if err != nil {
		t.Fatalf("RunProcessorOffline: %v", err)
	}
	if runErr != nil {
		t.Fatalf("перезагруженная дата не типизирована (КонецДня упал): %v", runErr)
	}
	if len(messages) != 1 || messages[0] != "Дата" {
		t.Fatalf("ТипЗнч(Об.Дата) = %v, ожидался [Дата]", messages)
	}
}
