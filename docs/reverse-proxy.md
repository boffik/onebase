# Обратный прокси: HTTPS, HTTP/2 и SSE

OneBase слушает **голый HTTP** (по умолчанию `127.0.0.1`, наружу — только явным
`--host`). TLS, HTTP/2 и раздача наружу делегированы обратному прокси
(nginx/Caddy/Traefik). Этот гайд — про то, как настроить прокси, чтобы под
нагрузкой на 100+ пользователей корректно работали live-обновления (SSE) и не
упирались в лимиты браузера.

## Почему это важно для SSE

Live-обновления (уведомления, авто-обновление списков) идут через **SSE** —
долгоживущее соединение `GET /ui/events`. Два узких места:

1. **Лимит браузера «6 соединений на origin» по HTTP/1.1.** Каждое SSE-соединение
   занимает целый слот из шести и держит его открытым постоянно. Несколько
   вкладок — и обычным XHR уже тесно. **Решение:** включить **HTTP/2 на стороне
   браузер↔прокси** (обычный TLS+HTTP/2). HTTP/2 мультиплексирует все запросы и
   SSE в одно соединение — лимит «6 на origin» снимается. Это самое важное.
2. **Буферизация прокси.** Если прокси буферизует ответ, события не доходят до
   браузера в реальном времени. OneBase сам ставит на `/ui/events` заголовок
   `X-Accel-Buffering: no` (nginx это уважает), но буферизацию для стрима лучше
   выключить и явно.

OneBase шлёт по SSE `: ping` каждые 25 с и закрывает простаивающие keep-alive
через 120 с (`IdleTimeout`) — таймауты прокси на этом эндпоинте должны быть
заведомо больше пинга.

## nginx

nginx НЕ умеет HTTP/2 к апстриму через `proxy_pass` (это только у `grpc_pass`),
поэтому к OneBase он ходит по HTTP/1.1 с keep-alive — этого достаточно. Главное —
HTTP/2 наружу и правильный `location /ui/events`.

```nginx
upstream onebase {
    server 127.0.0.1:8080;
    keepalive 32;                 # переиспользуем соединения к OneBase (в т.ч. при реконнектах SSE)
}

server {
    listen 443 ssl;
    http2 on;                     # HTTP/2 к браузеру — снимает лимит 6 соединений/origin для SSE
                                  # (nginx < 1.25.1: `listen 443 ssl http2;`)
    server_name onebase.example.com;

    ssl_certificate     /etc/ssl/onebase.crt;
    ssl_certificate_key /etc/ssl/onebase.key;

    # SSE-эндпоинт: без буферизации, длинный read-timeout, keep-alive к апстриму.
    location = /ui/events {
        proxy_pass http://onebase;
        proxy_http_version 1.1;
        proxy_set_header Connection "";       # включить keepalive к upstream
        proxy_buffering off;                  # не буферизировать событийный стрим
        proxy_cache off;
        proxy_read_timeout 1h;                # SSE живёт долго (пинг раз в 25 с)
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location / {
        proxy_pass http://onebase;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

За TLS не забудьте `ONEBASE_SECURE_COOKIES=true` (иначе session-cookie не пометится
`Secure`; см. `docs/DEPLOYMENT.md`).

## Caddy

Caddy автоматически отдаёт HTTP/2 наружу (при своём TLS) и не буферизует
`text/event-stream`, так что базовый конфиг уже SSE-дружелюбен:

```caddyfile
onebase.example.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080 {
        flush_interval -1         # мгновенный flush для стримов (SSE) — на всякий случай
    }
}
```

## Опционально: HTTP/2 без TLS к апстриму (h2c)

Плечо прокси↔OneBase по HTTP/1.1 держит **одно соединение на каждый SSE-поток**.
Go тянет тысячи таких без проблем, но если прокси умеет HTTP/2 к апстриму, можно
мультиплексировать множество SSE/XHR в несколько TCP-соединений к OneBase. Для
этого включите на OneBase **cleartext HTTP/2 (h2c)**:

```bash
ONEBASE_H2C=1        # в окружении службы (см. docs/DEPLOYMENT.md, «Параметры безопасности»)
```

В баннере старта появится строка `HTTP/2 без TLS (h2c) включён для апстрима`.
h2c обслуживается **поверх** HTTP/1.1: обычные HTTP/1.1-клиенты работают как
раньше, поэтому флаг безопасно включать заранее.

Настройка на стороне прокси, который поддерживает h2c к апстриму:

- **Caddy** — в блоке `reverse_proxy`:
  ```caddyfile
  reverse_proxy 127.0.0.1:8080 {
      transport http {
          versions h2c 2
      }
  }
  ```
- **Traefik** — у сервиса указать схему `h2c`:
  ```yaml
  services:
    onebase:
      loadBalancer:
        servers:
          - url: h2c://127.0.0.1:8080
  ```
- **nginx** — HTTP/2 к апстриму через `proxy_pass` не поддерживает; оставляйте
  HTTP/1.1 (раздел выше). `ONEBASE_H2C` тогда включать незачем.

### Замечание по безопасности

Держите `ONEBASE_H2C` **выключенным**, пока прокси перед OneBase не настроен
осознанно на HTTP/2 к апстриму. OneBase использует h2c по «prior knowledge»
(нативная поддержка stdlib), поэтому порт с включённым h2c не выставляйте
напрямую недоверенным клиентам — только за доверенным прокси. На саму авторизацию
OneBase h2c не влияет: запрос по h2c проходит тот же стек middleware (auth,
CSRF), что и по HTTP/1.1.

## Чек-лист

- [ ] TLS + **HTTP/2 наружу** (браузер↔прокси) — снимает лимит 6 conn/origin для SSE.
- [ ] `location /ui/events`: `proxy_buffering off`, `proxy_read_timeout` ≫ 25 с.
- [ ] keep-alive к апстриму (`upstream … keepalive`, `proxy_http_version 1.1`,
      пустой `Connection`).
- [ ] `ONEBASE_SECURE_COOKIES=true` за HTTPS.
- [ ] (Опционально) `ONEBASE_H2C=1` + h2c на прокси (Caddy/Traefik), если нужен
      HTTP/2 к апстриму. С nginx — не требуется.
