package ui

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/webassets"
)

// mountStatic регистрирует отдачу общих встроенных ассетов. Самохостинг вместо
// CDN: графики и редактор работают офлайн — десктопная база не должна зависеть
// от интернета. ECharts и Monaco вендорятся один раз в webassets и раздаются
// тем же путём, что и в конфигураторе лаунчера, чтобы рабочий стол и
// предпросмотр виджетов рисовались идентично.
func mountStatic(r chi.Router) {
	r.Handle("/vendor/echarts/*", http.StripPrefix("/vendor/echarts/", webassets.EChartsHandler()))
	// Monaco editor — инструменты разработчика (консоль кода/запросов, отладчик)
	// грузят его офлайн вместо CDN.
	r.Handle("/vendor/monaco/*", http.StripPrefix("/vendor/monaco/", webassets.MonacoHandler()))
	// SlickGrid — грид для редактируемых табличных частей managed-форм.
	r.Handle("/vendor/slickgrid/*", http.StripPrefix("/vendor/slickgrid/", webassets.SlickGridHandler()))
}
