package ui

import (
	"bytes"
	"embed"
	"net/http"
	"runtime/debug"

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

// MountPWA регистрирует маршруты PWA на ПУБЛИЧНОМ роутере (вне auth-группы).
// Это обязательно: браузер запрашивает manifest без credentials, а install-
// промпт и иконки фечатся вне контекста авторизованной страницы. Под auth-
// мидлварой эти запросы получали бы 401/redirect, и на любом инстансе с
// пользователями PWA не устанавливался бы. Ассеты не содержат пользовательских
// данных, поэтому публичная отдача безопасна. См. ревью PR #34 (план 45).
func (s *Server) MountPWA(r chi.Router) { mountPWA(r) }

// mountPWA регистрирует маршруты PWA. /sw.js обязан отдаваться из корня, иначе
// его scope не покроет /ui/* — поэтому маршруты в корне роутера, а не под
// /vendor. /sw.js и manifest с коротким кэшем (обновления должны доходить),
// иконки — immutable.
func mountPWA(r chi.Router) {
	r.Get("/manifest.webmanifest", servePWAFile("pwa/manifest.webmanifest", "application/manifest+json; charset=utf-8", "public, max-age=3600"))
	r.Get("/sw.js", serveSW())
	r.Get("/offline.html", servePWAFile("pwa/offline.html", "text/html; charset=utf-8", "no-cache"))
	r.Get("/icons/icon-192.png", servePWAFile("pwa/icons/icon-192.png", "image/png", "public, max-age=31536000, immutable"))
	r.Get("/icons/icon-512.png", servePWAFile("pwa/icons/icon-512.png", "image/png", "public, max-age=31536000, immutable"))
}

// serveSW отдаёт service worker, подставив в placeholder __OB_CACHE__ имя кэша
// с ревизией сборки. Так каждый релиз меняет имя кэша → старые кэши чистятся в
// activate, vendor-ассеты (URL не версионируются) перетягиваются заново. Без
// ручного bump'а константы. Подстановка делается один раз при создании хендлера.
func serveSW() http.HandlerFunc {
	data, err := pwaFS.ReadFile("pwa/sw.js")
	if err == nil {
		data = bytes.ReplaceAll(data, []byte("__OB_CACHE__"), []byte(swCacheName()))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	}
}

// swCacheName — имя кэша service worker, привязанное к ревизии git-сборки
// (debug.BuildInfo). Вне VCS-сборки (go test/run) ревизия пуста — используем
// стабильный фолбэк, чтобы значение было детерминированным.
func swCacheName() string {
	rev := "dev"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				rev = s.Value
				if len(rev) > 12 {
					rev = rev[:12]
				}
				break
			}
		}
	}
	return "onebase-" + rev
}
