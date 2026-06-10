package configcheck

// Валидация входящих REST-эндпоинтов (план 58): синтаксис yaml уже разобран
// загрузчиком, здесь — согласованность (дубли path, наличие обработчика).

import (
	"fmt"
	"strings"

	"github.com/ivantit66/onebase/internal/project"
)

// CheckEndpoints проверяет endpoints/*.yaml против загруженных модулей.
func CheckEndpoints(proj *project.Project) []Issue {
	var issues []Issue
	add := func(object, msg string) {
		issues = append(issues, Issue{File: "endpoints", Object: object, Kind: "Эндпоинт", Message: msg})
	}

	seenPath := map[string]string{} // lower path → имя эндпоинта
	for _, ep := range proj.Endpoints {
		if err := ep.Validate(); err != nil {
			add(ep.Name, err.Error())
			continue
		}
		low := strings.ToLower(ep.Path)
		if prev, dup := seenPath[low]; dup {
			add(ep.Name, fmt.Sprintf("путь %q уже занят эндпоинтом %q", ep.Path, prev))
		} else {
			seenPath[low] = ep.Name
		}

		prog, ok := proj.EndpointPrograms[strings.ToLower(ep.Handler)]
		if !ok {
			add(ep.Name, fmt.Sprintf("не найден модуль обработчика src/%s.endpoint.os", strings.ToLower(ep.Handler)))
			continue
		}
		found := false
		for _, p := range prog.Procedures {
			low := strings.ToLower(p.Name.Literal)
			if low == "обработать" || low == "handle" {
				found = true
				break
			}
		}
		if !found {
			add(ep.Name, fmt.Sprintf("в src/%s.endpoint.os нет процедуры Обработать(Запрос, Ответ)", strings.ToLower(ep.Handler)))
		}
	}
	return issues
}
