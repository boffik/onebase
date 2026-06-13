package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/ast"
)

// Регрессия (план критических фиксов, #2): HTTP-сервис, названный как
// одноимённый документ, не должен затирать модуль документа. Раньше
// src/<имя>.service.os и src/<имя>.posting.os писались в одну карту Programs,
// и сервис (идёт последним по алфавиту) перетирал ОбработкаПроведения —
// документ молча проводился без движений. Теперь сервис живёт в ServicePrograms.
func TestServiceDoesNotClobberDocumentPosting(t *testing.T) {
	dir := t.TempDir()
	mustMkdir := func(sub string) string {
		p := filepath.Join(dir, sub)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
		return p
	}
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	docDir := mustMkdir("documents")
	svcDir := mustMkdir("services")
	srcDir := mustMkdir("src")

	write(filepath.Join(docDir, "заказы.yaml"),
		"name: Заказы\nposting: true\nfields:\n  - name: Дата\n    type: date\n  - name: Сумма\n    type: number\n")
	write(filepath.Join(srcDir, "заказы.posting.os"),
		"Процедура ОбработкаПроведения()\nКонецПроцедуры\n")

	write(filepath.Join(svcDir, "заказы.yaml"),
		"name: Заказы\nroot_url: zakazy\nauth: none\ntemplates:\n  - template: /пинг\n    methods:\n      GET: Пинг\n")
	write(filepath.Join(srcDir, "заказы.service.os"),
		"Функция Пинг(Запрос) Экспорт\n  Возврат \"ok\";\nКонецФункции\n")

	proj, err := Load(dir)
	if err != nil {
		t.Fatalf("project.Load: %v", err)
	}
	defer proj.Close()

	hasProc := func(progs map[string]*ast.Program, key, proc string) bool {
		prog, ok := progs[key]
		if !ok {
			return false
		}
		for _, p := range prog.Procedures {
			if strings.EqualFold(p.Name.Literal, proc) {
				return true
			}
		}
		return false
	}

	if !hasProc(proj.Programs, "Заказы", "ОбработкаПроведения") {
		t.Errorf("ОбработкаПроведения документа потеряна — сервис затёр posting-модуль; Programs=%v", keys(proj.Programs))
	}
	if !hasProc(proj.ServicePrograms, "Заказы", "Пинг") {
		t.Errorf("обработчик сервиса Пинг не попал в ServicePrograms; ServicePrograms=%v", keys(proj.ServicePrograms))
	}
}

func keys(m map[string]*ast.Program) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
