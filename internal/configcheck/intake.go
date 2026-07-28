package configcheck

// Валидация входных шлюзов (план 90): наличие модуля-обработчика и процедуры
// Обработать, корректность и уникальность endpoint, а также коллизия с
// HTTP-сервисами (план 61) — приёмник и сервис делят префикс /hs/, приёмка
// матчится первой, поэтому endpoint в корне сервиса молча затенит сервис.
// Форму объявления (auth/secret/idempotency) проверяет metadata.Intake.Validate
// на загрузке; здесь — межобъектные проверки.

import (
	"fmt"
	"strings"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/project"
)

// CheckIntakes проверяет intake/*.yaml против модулей и HTTP-сервисов.
func CheckIntakes(proj *project.Project) []Issue {
	var issues []Issue
	add := func(object, msg string) {
		issues = append(issues, Issue{File: "intake", Object: object, Kind: "Входной шлюз", Message: msg})
	}

	modByLower := map[string]*ast.Program{}
	for name, prog := range proj.Modules {
		modByLower[strings.ToLower(name)] = prog
	}
	svcRoots := map[string]string{} // lower root_url → имя сервиса
	for _, svc := range proj.HTTPServices {
		svcRoots[strings.ToLower(strings.TrimSpace(svc.RootURL))] = svc.Name
	}

	seenEndpoint := map[string]string{}
	seenName := map[string]string{}
	for _, in := range proj.Intakes {
		if prev, dup := seenName[strings.ToLower(in.Name)]; dup {
			add(in.Name, fmt.Sprintf("имя шлюза дублируется (уже есть %q)", prev))
		} else {
			seenName[strings.ToLower(in.Name)] = in.Name
		}

		if in.Transport == "http" {
			if !strings.HasPrefix(in.Endpoint, "/hs/") {
				add(in.Name, fmt.Sprintf("endpoint %q должен начинаться с /hs/ — иначе маршрут не публикуется", in.Endpoint))
			} else {
				if prev, dup := seenEndpoint[in.Endpoint]; dup {
					add(in.Name, fmt.Sprintf("endpoint %q уже занят шлюзом %q", in.Endpoint, prev))
				} else {
					seenEndpoint[in.Endpoint] = in.Name
				}
				rest := strings.TrimPrefix(in.Endpoint, "/hs/")
				root := rest
				if i := strings.IndexByte(rest, '/'); i >= 0 {
					root = rest[:i]
				}
				if svc, clash := svcRoots[strings.ToLower(root)]; clash {
					add(in.Name, fmt.Sprintf("endpoint %q в корне HTTP-сервиса %q — шлюз затенит сервис (приёмка матчится первой)", in.Endpoint, svc))
				}
			}
		}

		if in.Handler != "" {
			prog, ok := modByLower[strings.ToLower(in.Handler)]
			if !ok {
				add(in.Name, fmt.Sprintf("не найден модуль обработчика src/%s.module.os", strings.ToLower(in.Handler)))
			} else {
				found := false
				for _, p := range prog.Procedures {
					if strings.EqualFold(p.Name.Literal, "Обработать") {
						found = true
						break
					}
				}
				if !found {
					add(in.Name, fmt.Sprintf("в модуле %s нет процедуры Обработать(Конверт)", in.Handler))
				}
			}
		}
	}
	return issues
}
