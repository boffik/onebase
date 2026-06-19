package ui

// Рантайм-настройки отчёта (план 70): чтение пользовательских настроек из
// запроса и вычисление эффективной компоновки. Источник правок — панель
// «Настройки» на форме отчёта, которая пишет скрытое поле __settings (JSON
// report.UserReportSettings).

import (
	"net/http"

	reportpkg "github.com/ivantit66/onebase/internal/report"
)

// readReportSettings разбирает пользовательские настройки из поля __settings
// запроса (FormValue читает и POST-форму, и GET-query). Пустое или повреждённое
// значение → nil (поведение отчёта по умолчанию).
func readReportSettings(r *http.Request) *reportpkg.UserReportSettings {
	raw := r.FormValue("__settings")
	if raw == "" {
		return nil
	}
	s, err := reportpkg.ParseUserSettings(raw)
	if err != nil {
		return nil
	}
	return s
}

// effectiveComposition вычисляет компоновку, по которой строится отчёт.
// Приоритет: пользовательский override (settings.Composition) → выбранный
// вариант (settings.Variant) → основной composition конфигурации.
func effectiveComposition(rep *reportpkg.Report, s *reportpkg.UserReportSettings) *reportpkg.Composition {
	if s != nil && s.Composition != nil {
		return s.Composition
	}
	if s != nil {
		return rep.ActiveComposition(s.Variant)
	}
	return rep.ActiveComposition("")
}
