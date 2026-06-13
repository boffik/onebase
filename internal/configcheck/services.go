package configcheck

// Валидация HTTP-сервисов (план 61): дубли корневых URL, наличие модуля и
// процедур-обработчиков, согласованность аутентификации (token/hmac требуют
// секрет — поглощено из плана 58).

import (
	"fmt"
	"strings"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/project"
)

// CheckHTTPServices проверяет services/*.yaml против загруженных модулей.
func CheckHTTPServices(proj *project.Project) []Issue {
	var issues []Issue
	add := func(object, msg string) {
		issues = append(issues, Issue{File: "services", Object: object, Kind: "HTTP-сервис", Message: msg})
	}

	// ServicePrograms ключуется капитализированным именем файла — ищем
	// регистронезависимо. Сервисы хранятся отдельно от Programs (план 61),
	// чтобы не конфликтовать с модулем одноимённого документа.
	progByLower := map[string]*ast.Program{}
	for name, prog := range proj.ServicePrograms {
		progByLower[strings.ToLower(name)] = prog
	}

	seenRoot := map[string]string{}
	for _, svc := range proj.HTTPServices {
		if strings.TrimSpace(svc.Name) == "" {
			add("(без имени)", "не задано имя сервиса (name)")
			continue
		}
		if strings.TrimSpace(svc.RootURL) == "" {
			add(svc.Name, "не задан корневой URL (root_url)")
			continue
		}
		low := strings.ToLower(svc.RootURL)
		if prev, dup := seenRoot[low]; dup {
			add(svc.Name, fmt.Sprintf("корневой URL %q уже занят сервисом %q", svc.RootURL, prev))
		} else {
			seenRoot[low] = svc.Name
		}

		switch svc.Auth {
		case "", "none", "basic", "session":
		case "token", "hmac":
			if strings.TrimSpace(svc.Secret) == "" {
				add(svc.Name, fmt.Sprintf("auth %q требует secret (используйте ${env:VAR})", svc.Auth))
			}
		default:
			add(svc.Name, fmt.Sprintf("неизвестный auth %q (none|basic|session|token|hmac)", svc.Auth))
		}

		prog, ok := progByLower[strings.ToLower(svc.Name)]
		if !ok {
			add(svc.Name, fmt.Sprintf("не найден модуль обработчиков src/%s.service.os", strings.ToLower(svc.Name)))
			continue
		}
		procs := map[string]bool{}
		for _, p := range prog.Procedures {
			procs[strings.ToLower(p.Name.Literal)] = true
		}
		for _, t := range svc.Templates {
			for method, handler := range t.Methods {
				if handler == "" || !procs[strings.ToLower(handler)] {
					add(svc.Name, fmt.Sprintf("шаблон %q (%s): обработчик %q не найден в src/%s.service.os",
						t.Template, method, handler, strings.ToLower(svc.Name)))
				}
			}
		}
	}
	return issues
}
