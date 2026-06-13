package configcheck

// Тесты валидации HTTP-сервисов (план 61).

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/ivantit66/onebase/internal/httpservice"
	"github.com/ivantit66/onebase/internal/project"
)

func parseServiceProg(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, err := parser.New(lexer.New(src, "test.service.os")).ParseProgram()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return prog
}

func TestCheckHTTPServices(t *testing.T) {
	good := parseServiceProg(t, `Функция ПолучитьЗаказ(Запрос) Экспорт
  Возврат "ok";
КонецФункции`)

	mk := func(name, root, auth, secret string, tmpl httpservice.URLTemplate) *httpservice.Service {
		s := &httpservice.Service{Name: name, RootURL: root, Auth: auth, Secret: secret,
			Templates: []httpservice.URLTemplate{tmpl}}
		s.Normalize()
		return s
	}
	okTmpl := httpservice.URLTemplate{Template: "/{id}", Methods: map[string]string{"GET": "ПолучитьЗаказ"}}
	badTmpl := httpservice.URLTemplate{Template: "/{id}", Methods: map[string]string{"GET": "НетТакого"}}

	proj := &project.Project{
		HTTPServices: []*httpservice.Service{
			mk("Заказы", "orders", "none", "", okTmpl),
			mk("Дубль", "orders", "none", "", okTmpl),       // дубль root_url
			mk("БезМодуля", "nomod", "none", "", okTmpl),    // нет src-модуля
			mk("Заказы2", "orders2", "none", "", badTmpl),   // нет процедуры
			mk("БезСекрета", "tok", "token", "", okTmpl),    // token без секрета
			mk("Странный", "weird", "странный", "", okTmpl), // неизвестный auth
		},
		ServicePrograms: map[string]*ast.Program{
			"Заказы":     good,
			"Дубль":      good,
			"Заказы2":    good,
			"БезСекрета": good,
			"Странный":   good,
		},
	}

	issues := CheckHTTPServices(proj)
	for _, want := range []string{
		"уже занят",
		"не найден модуль",
		"не найден в src/заказы2.service.os",
		"требует secret",
		"неизвестный auth",
	} {
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
	if len(issues) != 5 {
		t.Errorf("ожидалось ровно 5 ошибок, получено %d: %+v", len(issues), issues)
	}
}
