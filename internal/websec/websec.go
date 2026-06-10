package websec

// Security-заголовки и CSRF-защита (план 53, этап 3; анализ §2.5).
//
// CSRF — проверка Origin/Sec-Fetch-Site вместо double-submit cookie:
// современные браузеры шлют Origin на каждый cross-origin (и Chromium — на
// каждый same-origin) POST, а Sec-Fetch-Site — на любой запрос. Несовпадение
// origin с Host → 403. Запросы без обоих заголовков (curl, серверные
// интеграции, старые скрипты) пропускаются — REST-клиенты не ломаются, а
// браузерная атака без Origin невозможна. Это рекомендованная OWASP
// альтернатива токенам, не требующая правки всех форм/fetch в шаблонах.

import (
	"net/http"
	"net/url"
	"strings"
)

// securityHeaders добавляет базовые защитные заголовки ко всем ответам.
// X-Frame-Options не используется: конфигуратор (лаунчер) живёт на другом
// порту (другой origin) и встраивает базу в iframe — SAMEORIGIN сломал бы
// его. CSP frame-ancestors допускает self и локальные адреса любого порта,
// закрывая clickjacking с внешних сайтов.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		// Режет утечку URL (в т.ч. одноразовых кодов bootstrap) через Referer
		// на внешние ресурсы.
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy",
			"frame-ancestors 'self' http://localhost:* http://127.0.0.1:*")
		next.ServeHTTP(w, r)
	})
}

// csrfProtect отклоняет мутирующие запросы с чужим Origin.
func CSRFProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" {
			if !sameOrigin(origin, r.Host) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
		} else if sfs := r.Header.Get("Sec-Fetch-Site"); sfs != "" {
			// Браузер без Origin, но с Fetch Metadata: допускаем только
			// same-origin/same-site/none (none = прямой заход пользователя).
			switch strings.ToLower(sfs) {
			case "same-origin", "same-site", "none":
			default:
				http.Error(w, "cross-site request rejected", http.StatusForbidden)
				return
			}
		}
		// Ни Origin, ни Sec-Fetch-Site — не браузер (curl/интеграции): пропускаем.
		next.ServeHTTP(w, r)
	})
}

// sameOrigin сравнивает host:port из Origin-заголовка с Host запроса.
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false // включая Origin: null
	}
	return strings.EqualFold(u.Host, host)
}
