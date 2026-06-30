package configcheck

import (
	"fmt"
	"strings"

	"github.com/ivantit66/onebase/internal/project"
)

// validReportOutputFormats — допустимые значения output_format отчёта.
var validReportOutputFormats = map[string]bool{"": true, "html": true, "pdf": true, "excel": true}

// CheckReportOutputFormat проверяет, что output_format отчёта (issue #218) — одно
// из html|pdf|excel (или пусто). Поле распознаётся загрузчиком, поэтому неизвестное
// значение — почти наверняка опечатка; ловим её на check как ошибку, а не молча
// игнорируем (иначе автор получает «ложную уверенность», как было с #219).
func CheckReportOutputFormat(proj *project.Project) []Issue {
	var issues []Issue
	for _, rep := range proj.Reports {
		of := strings.ToLower(strings.TrimSpace(rep.OutputFormat))
		if validReportOutputFormats[of] {
			continue
		}
		issues = append(issues, Issue{
			File:         "reports/" + rep.Name + ".yaml",
			Object:       rep.Name,
			Kind:         "Отчёт",
			Code:         "report.bad-output-format",
			Message:      fmt.Sprintf("неизвестный output_format %q: допустимо html, pdf или excel", rep.OutputFormat),
			SuggestedFix: "Используйте output_format: html|pdf|excel либо уберите ключ.",
		})
	}
	return issues
}
