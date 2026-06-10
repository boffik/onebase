package configcheck

// Тесты валидации входящих эндпоинтов (план 58).

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/project"
)

func parseProg(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, err := parser.New(lexer.New(src, "test.endpoint.os")).ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return prog
}

func TestCheckEndpoints(t *testing.T) {
	good := parseProg(t, `Процедура Обработать(Запрос, Ответ) Экспорт
КонецПроцедуры`)
	noHandle := parseProg(t, `Процедура ЧтоТоДругое()
КонецПроцедуры`)

	proj := &project.Project{
		Endpoints: []*metadata.Endpoint{
			{Name: "Ок", Path: "/hooks/a", Method: "POST", Auth: "none", Handler: "ок"},
			{Name: "Дубль", Path: "/hooks/a", Method: "POST", Auth: "none", Handler: "ок"},
			{Name: "БезМодуля", Path: "/hooks/b", Method: "POST", Auth: "none", Handler: "нет_такого"},
			{Name: "БезПроцедуры", Path: "/hooks/c", Method: "POST", Auth: "none", Handler: "пустой"},
			{Name: "БезСекрета", Path: "/hooks/d", Method: "POST", Auth: "token", Handler: "ок"},
		},
		EndpointPrograms: map[string]*ast.Program{
			"ок":     good,
			"пустой": noHandle,
		},
	}

	issues := CheckEndpoints(proj)
	wantSubstrings := []string{
		"уже занят",                 // дубль path
		"не найден модуль",          // отсутствующий handler-модуль
		"нет процедуры Обработать",  // модуль без процедуры
		"требует secret",            // token без секрета
	}
	for _, want := range wantSubstrings {
		found := false
		for _, is := range issues {
			if strings.Contains(is.Message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("не найдена ожидаемая ошибка %q среди: %+v", want, issues)
		}
	}
	if len(issues) != 4 {
		t.Errorf("ожидалось ровно 4 ошибки, получено %d: %+v", len(issues), issues)
	}
}
