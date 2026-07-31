package api

import "net/http"

// h2cEnabled сообщает, включён ли обмен по HTTP/2 без TLS (cleartext, h2c) с
// апстримом — через переменную окружения ONEBASE_H2C (план 111, P2-1).
//
// По умолчанию ВЫКЛЮЧЕНО: включайте только когда обратный прокси перед OneBase
// осознанно настроен говорить с апстримом по HTTP/2 (Caddy `versions h2c 2`,
// Traefik `scheme: h2c`). Тогда прокси мультиплексирует множество SSE/XHR в
// несколько TCP-соединений к OneBase вместо connection-на-поток по HTTP/1.1.
// Браузерный лимит «6 соединений на origin», который душит SSE, снимается
// отдельно — включением HTTP/2 на стороне браузер↔прокси (обычный TLS+HTTP/2).
// Подробности и примеры прокси — в docs/reverse-proxy.md.
//
// Держим h2c opt-in намеренно: это смена транспортного контракта, уместная лишь
// когда HTTP/2 к апстриму реально настроен. Порт OneBase с h2c не выставляйте
// напрямую недоверенным клиентам — только за таким прокси.
func h2cEnabled() bool { return envBool("ONEBASE_H2C") }

// configureH2C включает на сервере cleartext HTTP/2 (h2c, prior knowledge) поверх
// HTTP/1.1 через нативный http.Server.Protocols (stdlib, Go 1.24+). При enabled ==
// false не трогает srv: Protocols остаётся nil, и на plain-listener сервер
// обслуживает только HTTP/1.1 — поведение существующих установок не меняется.
//
// h2c включается на самом сервере, а не оборачиванием handler, поэтому таймауты
// http.Server (IdleTimeout и пр.) применяются к h2c-соединениям штатно. Обычные
// HTTP/1.1-клиенты не затронуты; запрос по h2c идёт через тот же стек middleware
// роутера (auth/CSRF) — h2c не в обход авторизации OneBase.
func configureH2C(srv *http.Server, enabled bool) {
	if !enabled {
		return
	}
	p := new(http.Protocols)
	p.SetHTTP1(true)            // сохраняем HTTP/1.1
	p.SetUnencryptedHTTP2(true) // h2c: HTTP/2 без TLS на том же порту
	srv.Protocols = p
}
