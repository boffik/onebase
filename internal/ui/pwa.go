package ui

import (
	"embed"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// pwaFS — встроенные ассеты PWA (этап 45): манифест, service worker, офлайн-
// заглушка и иконки. Самохостинг, как и остальная статика onebase — работает
// без интернета. gen.go (генератор иконок) помечен build-тегом ignore и в
// embed-набор не попадает, поэтому перечисляем файлы явно.
//
//go:embed pwa/manifest.webmanifest pwa/sw.js pwa/offline.html pwa/icons/icon-192.png pwa/icons/icon-512.png
var pwaFS embed.FS

// servePWAFile отдаёт один встроенный файл с заданными Content-Type и
// Cache-Control. Содержимое читается один раз при создании хендлера (embed
// статичен), поэтому per-request аллокаций нет.
func servePWAFile(name, contentType, cacheControl string) http.HandlerFunc {
	data, err := pwaFS.ReadFile(name)
	return func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		if cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		_, _ = w.Write(data)
	}
}

// mountPWA регистрирует маршруты PWA. /sw.js обязан отдаваться из корня, иначе
// его scope не покроет /ui/* — поэтому маршруты в корне роутера, а не под
// /vendor. /sw.js и manifest с коротким кэшем (обновления должны доходить),
// иконки — immutable.
func mountPWA(r chi.Router) {
	r.Get("/manifest.webmanifest", servePWAFile("pwa/manifest.webmanifest", "application/manifest+json; charset=utf-8", "public, max-age=3600"))
	r.Get("/sw.js", servePWAFile("pwa/sw.js", "application/javascript; charset=utf-8", "no-cache"))
	r.Get("/offline.html", servePWAFile("pwa/offline.html", "text/html; charset=utf-8", "no-cache"))
	r.Get("/icons/icon-192.png", servePWAFile("pwa/icons/icon-192.png", "image/png", "public, max-age=31536000, immutable"))
	r.Get("/icons/icon-512.png", servePWAFile("pwa/icons/icon-512.png", "image/png", "public, max-age=31536000, immutable"))
}
