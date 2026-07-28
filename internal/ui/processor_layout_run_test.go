package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
)

// writeProcLayoutProject создаёт минимальный файловый проект с обработкой,
// чья процедура Выполнить() обращается к Макет.Область("Заголовок"). Если
// withLayout=false, файл макета не создаётся (для негативной проверки).
func writeProcLayoutProject(t *testing.T, procOS string, withLayout bool) string {
	t.Helper()
	dir := t.TempDir()
	mk := func(sub string) string {
		p := filepath.Join(dir, sub)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
		return p
	}
	procDir := mk("processors")
	srcDir := mk("src")

	if err := os.WriteFile(filepath.Join(procDir, "загрузкакурсов.yaml"),
		[]byte("name: ЗагрузкаКурсов\ntitle: Загрузка курсов\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "загрузкакурсов.proc.os"), []byte(procOS), 0o644); err != nil {
		t.Fatal(err)
	}
	if withLayout {
		layout := "name: ЗагрузкаКурсов\nareas:\n  Заголовок:\n    rows:\n      - cells:\n          - text: \"Отчёт\"\n"
		if err := os.WriteFile(filepath.Join(srcDir, "загрузкакурсов.proc.layout.yaml"), []byte(layout), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestProcessorLayoutInjectedInProcrun — e2e для issue #48 пп.1-2: обработка,
// загруженная из файлового проекта, в пути запуска procrun
// (ui.RunProcessorOffline) видит переменную «Макет» и может получить область
// макета. Без инъекции тест падал бы с ошибкой «неизвестная переменная Макет».
func TestProcessorLayoutInjectedInProcrun(t *testing.T) {
	// Обращаемся к Макет.Область("Заголовок") и проверяем, что область реально
	// получена. Без инъекции «Макет» интерпретатор молча отдаёт nil (вызов
	// метода на неразрешённом идентификаторе → nil), поэтому ЗначениеЗаполнено
	// вернёт Ложь и обработка завершится исключением — тест станет красным.
	procOS := "Процедура Выполнить()\n" +
		"  Обл = Макет.Область(\"Заголовок\");\n" +
		"  Если НЕ ЗначениеЗаполнено(Обл) Тогда\n" +
		"    ВызватьИсключение(\"Макет не инжектирован: область не получена\");\n" +
		"  КонецЕсли;\n" +
		"  Сообщить(\"ok\");\n" +
		"КонецПроцедуры\n"
	dir := writeProcLayoutProject(t, procOS, true)

	ctx := context.Background()
	proj, err := project.Load(dir)
	if err != nil {
		t.Fatalf("project.Load: %v", err)
	}
	defer proj.Close()

	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()

	messages, runErr, err := RunProcessorOffline(ctx, proj, db, "ЗагрузкаКурсов", nil, nil)
	if err != nil {
		t.Fatalf("RunProcessorOffline: %v", err)
	}
	if runErr != nil {
		t.Fatalf("ошибка выполнения обработки (Макет не инжектирован?): %v", runErr)
	}
	if len(messages) != 1 || messages[0] != "ok" {
		t.Fatalf("ожидалось сообщение [ok], получено %v", messages)
	}
}

// TestProcessorWithoutLayoutStillRuns — без файла макета обращение к Макет даёт
// ошибку (переменная не инжектируется), но обработка без обращения к Макет
// продолжает работать как прежде.
func TestProcessorWithoutLayoutStillRuns(t *testing.T) {
	procOS := "Процедура Выполнить()\n  Сообщить(\"ok\");\nКонецПроцедуры\n"
	dir := writeProcLayoutProject(t, procOS, false)

	ctx := context.Background()
	proj, err := project.Load(dir)
	if err != nil {
		t.Fatalf("project.Load: %v", err)
	}
	defer proj.Close()
	if proj.Processors[0].Layout != nil {
		t.Fatal("без файла макета Layout должен быть nil")
	}

	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()

	messages, runErr, err := RunProcessorOffline(ctx, proj, db, "ЗагрузкаКурсов", nil, nil)
	if err != nil {
		t.Fatalf("RunProcessorOffline: %v", err)
	}
	if runErr != nil {
		t.Fatalf("обработка без макета должна выполняться без ошибки: %v", runErr)
	}
	if len(messages) != 1 || !strings.Contains(messages[0], "ok") {
		t.Fatalf("ожидалось сообщение ok, получено %v", messages)
	}
}

func TestRunProcessorOffline_ReturnsDocumentHookMessages(t *testing.T) {
	procOS := `Процедура Выполнить()
  Док = Документы.СообщениеХука.Создать();
  Док.Номер = "1";
  Ссылка = Док.Записать();
  ЗагруженныйДокумент = Ссылка.ПолучитьОбъект();
  ЗагруженныйДокумент.Провести();
КонецПроцедуры
`
	dir := writeProcLayoutProject(t, procOS, false)
	docDir := filepath.Join(dir, "documents")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	docYAML := "name: СообщениеХука\nposting: true\nfields:\n  - name: Номер\n    type: string\n"
	if err := os.WriteFile(filepath.Join(docDir, "сообщениехука.yaml"), []byte(docYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	onWriteOS := `Процедура ПриЗаписи()
  Сообщить("из ПриЗаписи");
КонецПроцедуры
`
	if err := os.WriteFile(filepath.Join(dir, "src", "сообщениехука.os"), []byte(onWriteOS), 0o644); err != nil {
		t.Fatal(err)
	}
	onPostOS := `Процедура ОбработкаПроведения()
  Сообщить("из ОбработкиПроведения");
КонецПроцедуры
`
	if err := os.WriteFile(filepath.Join(dir, "src", "сообщениехука.posting.os"), []byte(onPostOS), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	proj, err := project.Load(dir)
	if err != nil {
		t.Fatalf("project.Load: %v", err)
	}
	defer proj.Close()

	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, proj.Entities); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	messages, runErr, err := RunProcessorOffline(ctx, proj, db, "ЗагрузкаКурсов", nil, nil)
	if err != nil {
		t.Fatalf("RunProcessorOffline: %v", err)
	}
	if runErr != nil {
		t.Fatalf("ошибка выполнения обработки: %v", runErr)
	}
	wantMessages := []string{"из ПриЗаписи", "из ПриЗаписи", "из ОбработкиПроведения"}
	if len(messages) != len(wantMessages) {
		t.Fatalf("сообщения обработки = %v, ожидались %v", messages, wantMessages)
	}
	for i, want := range wantMessages {
		if messages[i] != want {
			t.Fatalf("сообщения обработки = %v, ожидались %v", messages, wantMessages)
		}
	}
}
