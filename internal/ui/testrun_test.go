package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
)

// writeTestRunnerProject создаёт файловый проект с набором обработок: обычной,
// проходящим тестом, проваленным тестом, тестом с ошибкой выполнения и пустым
// тестом без проверок. Каждая пара — YAML в processors/ и код в src/.
func writeTestRunnerProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	procDir := filepath.Join(dir, "processors")
	srcDir := filepath.Join(dir, "src")
	for _, d := range []string{procDir, srcDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	add := func(name, kind, code string) {
		yaml := "name: " + name + "\n"
		if kind != "" {
			yaml += "kind: " + kind + "\n"
		}
		low := name // имена ASCII в этом тесте — путь src совпадает с именем в нижнем регистре
		if err := os.WriteFile(filepath.Join(procDir, low+".yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, low+".proc.os"), []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	add("Normal", "", "Процедура Выполнить()\n  Сообщить(\"не тест\");\nКонецПроцедуры\n")
	add("PassTest", "test", "Процедура Выполнить()\n"+
		"  Утверждать.Равно(2 + 2, 4, \"арифметика\");\n"+
		"  Утверждать.Истина(1 = 1, \"истина\");\n"+
		"  Утверждать.Заполнено(\"x\", \"заполнено\");\n"+
		"КонецПроцедуры\n")
	add("FailTest", "test", "Процедура Выполнить()\n"+
		"  Утверждать.Равно(2 + 2, 5, \"неверная арифметика\");\n"+
		"  Утверждать.Истина(1 = 1, \"эта пройдёт\");\n"+
		"КонецПроцедуры\n")
	add("ErrorTest", "test", "Процедура Выполнить()\n"+
		"  Утверждать.Истина(Истина, \"до ошибки\");\n"+
		"  ВызватьИсключение(\"бум\");\n"+
		"КонецПроцедуры\n")
	add("EmptyTest", "test", "Процедура Выполнить()\n  Сообщить(\"без проверок\");\nКонецПроцедуры\n")
	return dir
}

func loadTestRunnerProject(t *testing.T) (*project.Project, *storage.DB) {
	t.Helper()
	dir := writeTestRunnerProject(t)
	ctx := context.Background()
	proj, err := project.Load(dir)
	if err != nil {
		t.Fatalf("project.Load: %v", err)
	}
	t.Cleanup(func() { proj.Close() })
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return proj, db
}

func caseByName(res TestRunResult, name string) *TestCaseResult {
	for i := range res.Cases {
		if res.Cases[i].Name == name {
			return &res.Cases[i]
		}
	}
	return nil
}

func TestRunTests_DiscoversOnlyKindTest(t *testing.T) {
	proj, db := loadTestRunnerProject(t)
	res, err := RunTests(context.Background(), proj, db, "")
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	// Обычная обработка Normal не должна попасть в прогон.
	if caseByName(res, "Normal") != nil {
		t.Fatal("обычная обработка (без kind: test) не должна выполняться раннером")
	}
	if len(res.Cases) != 4 {
		names := make([]string, len(res.Cases))
		for i, c := range res.Cases {
			names[i] = c.Name
		}
		t.Fatalf("ожидалось 4 теста, получено %d: %v", len(res.Cases), names)
	}
	// Порядок детерминирован (по имени).
	if res.Cases[0].Name != "EmptyTest" {
		t.Fatalf("тесты должны идти по имени, первым ожидался EmptyTest, получен %s", res.Cases[0].Name)
	}
}

func TestRunTests_PassFailErrorEmpty(t *testing.T) {
	proj, db := loadTestRunnerProject(t)
	res, err := RunTests(context.Background(), proj, db, "")
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	if res.OK() {
		t.Fatal("прогон с провалами/ошибками должен быть не-OK")
	}

	pass := caseByName(res, "PassTest")
	if pass == nil || !pass.OK() || pass.Passed != 3 || pass.Failed != 0 {
		t.Fatalf("PassTest: ожидалось 3 успешных проверки, получено %+v", pass)
	}

	fail := caseByName(res, "FailTest")
	if fail == nil || fail.OK() || fail.Passed != 1 || fail.Failed != 1 {
		t.Fatalf("FailTest: ожидалось 1 успех + 1 провал, получено %+v", fail)
	}

	errc := caseByName(res, "ErrorTest")
	if errc == nil || errc.OK() || errc.Err == nil {
		t.Fatalf("ErrorTest: ожидалась ошибка выполнения, получено %+v", errc)
	}
	// Проверки до ВызватьИсключение должны учитываться.
	if errc.Passed != 1 {
		t.Fatalf("ErrorTest: ожидалась 1 проверка до ошибки, получено %d", errc.Passed)
	}

	empty := caseByName(res, "EmptyTest")
	if empty == nil || empty.OK() {
		t.Fatal("EmptyTest без проверок должен считаться неуспешным")
	}

	tests, passedTests, asserts, failedAsserts := res.Totals()
	if tests != 4 || passedTests != 1 {
		t.Fatalf("итоги: tests=%d passed=%d, ожидалось 4/1", tests, passedTests)
	}
	if asserts != 6 || failedAsserts != 1 {
		t.Fatalf("итоги проверок: asserts=%d failed=%d, ожидалось 6/1", asserts, failedAsserts)
	}
}

func TestRunTests_FilterByName(t *testing.T) {
	proj, db := loadTestRunnerProject(t)
	res, err := RunTests(context.Background(), proj, db, "pass")
	if err != nil {
		t.Fatalf("RunTests: %v", err)
	}
	if len(res.Cases) != 1 || res.Cases[0].Name != "PassTest" {
		t.Fatalf("фильтр pass должен оставить только PassTest, получено %+v", res.Cases)
	}
	if !res.OK() {
		t.Fatal("прогон только PassTest должен быть OK")
	}
}
