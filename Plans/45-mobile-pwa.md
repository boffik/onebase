# Этап 45 — Мобильный доступ: адаптивная вёрстка + PWA

## Контекст

UI onebase рассчитан только на десктоп: серверный рендеринг (Go `html/template`),
вся вёрстка в `internal/ui/templates.go`, **нет** `<meta viewport>`, нет
media-queries, сайдбар жёстко 210px, таблицы под ширину ≥900px. На телефоне это
неудобно: мелко, горизонтальный скролл всей страницы, недоступное боковое меню.

Цель — комфортно пользоваться приложением с телефона, **не ломая десктоп**.
Подход: адаптивная вёрстка (responsive) + PWA (установка на домашний экран,
полноэкранный запуск, быстрый старт). Сервер уже опубликован под HTTPS
(`https://demo.ivantitov.tech`) — это закрывает обязательное для PWA требование
secure context; на десктопе через `localhost` service worker тоже разрешён.

**Десктоп не меняется.** Все мобильные стили живут внутри
`@media (max-width: …)` и применяются только на узких экранах — на широком экране
вёрстка остаётся прежней байт-в-байт. PWA-слой (manifest + service worker)
аддитивный: на десктопе лишь добавляет «установить приложение», ничего не меняя в
отрисовке.

Приоритет экранов: списки и формы, дашборд/виджеты, отчёты и регистры. Админка и
инструменты разработчика — вне приоритета (получат адаптив «бесплатно» от общих
правил, точечно не дорабатываются).

Принцип: прогрессивное улучшение поверх существующего SSR. **Без новых
зависимостей и без шага сборки фронтенда** — только правки Go-шаблонов и пара
новых статических маршрутов.

## Синтаксис / поведение для пользователя

- На телефоне боковое меню скрыто и открывается кнопкой-гамбургером (☰) в
  верхней панели; закрывается тапом по затемнению или по выбору пункта.
- Формы — в одну колонку, крупные тач-цели; широкие таблицы списков/отчётов
  скроллятся по горизонтали внутри своей области (страница не «едет» целиком).
- Через меню браузера «Добавить на главный экран» приложение ставится как PWA:
  своя иконка, запуск на весь экран, быстрый старт за счёт кэша оболочки.
- При отсутствии сети показывается простая страница-заглушка `/offline.html`.

## Хранилище / SQL

Не применимо (изменений схемы БД нет).

## Изменения в коде

### 1. `<head>` и мета — `internal/ui/templates.go`, `tplHead` (стр. 308+)

После `<meta charset="UTF-8">` (стр. 310) добавить:
- `<meta name="viewport" content="width=device-width, initial-scale=1">` —
  главный однострочный выигрыш, безопасен для десктопа.
- `<meta name="theme-color" content="#1e293b">` (цвет topbar).
- `<link rel="manifest" href="/manifest.webmanifest">`.
- `<link rel="apple-touch-icon" href="/icons/icon-192.png">`,
  `<meta name="apple-mobile-web-app-capable" content="yes">`,
  `<meta name="apple-mobile-web-app-title" content="onebase">`.

В конце `tplHead` (рядом с inline-`<script>` панели сообщений) — guard-регистрация
SW:
```html
<script>if('serviceWorker'in navigator){window.addEventListener('load',function(){navigator.serviceWorker.register('/sw.js').catch(function(){});});}</script>
```

### 2. Мобильная CSS — единый `@media`-блок в `tplHead` (перед `</style>`, ~стр. 411)

Блок `@media (max-width: 820px){ … }`, активный только на узких экранах:
- `.app-body{display:block;overflow:visible}` — снять desktop-flex с фикс-высотой.
- `aside` → выезжающий drawer: `position:fixed;left:0;top:0;bottom:0;z-index:400;`
  `transform:translateX(-100%);transition:transform .2s;` +
  `body.nav-open aside{transform:translateX(0)}`.
- Затемняющий оверлей `body.nav-open::before{…}` (закрытие по тапу).
- `main{padding:14px}`; `.card{padding:16px}`; `.row-top{flex-wrap:wrap;gap:8px}`.
- Тач-цели: `aside a{padding:10px 14px}`, `.btn{padding:10px 18px}`.
- `.nav-toggle{display:inline-flex}` (по умолчанию `display:none` на десктопе).

`.dash-row` (`tplIndex`) уже `flex-wrap` с `flex:1 1 220px` — карточки сами встанут
в колонку; ECharts подстроится под ширину контейнера. Доработка не нужна.

### 3. Гамбургер-меню — `tplNav` (`internal/ui/templates.go`, стр. 457+)

В `<header class="topbar">` перед `topbar-title` — кнопка (видна только на мобиле):
```html
<button class="nav-toggle" aria-label="Меню" onclick="document.body.classList.toggle('nav-open')">&#9776;</button>
```
Плюс небольшой JS: закрывать drawer по клику на оверлей и при переходе по `aside a`
(снимать класс `nav-open`). Логику `localStorage` для `details.navsec`
(стр. 514-527) не трогаем — совместима.

### 4. Таблицы списков/отчётов/регистров — адаптив

Дёшево и без правки каждой `<td>`: в мобильном `@media` сделать широкие таблицы
горизонтально скроллящимися одним правилом на родителе, напр.
`main table{display:block;overflow-x:auto;white-space:nowrap}`. Покрывает
`tplList`, `tplReport`, `tplRegister`, `tplInfoReg`, `tplAccountReg`. (Вариант
«карточки» через `td::before{content:attr(data-label)}` требует `data-label` в
каждой ячейке шаблонов — оставляем на потом, если скролла окажется мало.)

### 5. Управляемые формы — `internal/ui/templates_managed.go`

В CSS-блок `tplManagedForm` добавить аналогичный `@media (max-width:820px)`:
form-group в одну колонку, табы с горизонтальным скроллом, ТЧ-таблицы
(`.tp-table`) в скролл-контейнер. Обычные формы (`tplForm`) уже full-width inputs +
`grid auto-fill minmax(200px,1fr)` для фильтров — схлопнутся сами.

### 6. PWA-ассеты — новый `internal/ui/pwa.go` + правка `internal/ui/static.go`

По образцу `internal/webassets/assets.go` (`go:embed` + `http.FileServer`):
- `//go:embed pwa/manifest.webmanifest pwa/sw.js pwa/offline.html pwa/icons/*`
  (новый каталог `internal/ui/pwa/`).
- Маршруты в `mountStatic` (`internal/ui/static.go:15`):
  - `GET /manifest.webmanifest` → `Content-Type: application/manifest+json`.
  - `GET /sw.js` → `Content-Type: application/javascript`,
    `Cache-Control: no-cache`. **Файл обязан отдаваться из корня** (scope `/`),
    иначе SW не контролирует `/ui/*`.
  - `GET /icons/*` → `Cache-Control: public, max-age=31536000, immutable`.
  - `GET /offline.html` → страница-заглушка офлайна.

**`manifest.webmanifest`**: `name`/`short_name` = onebase, `start_url:"/ui/"`,
`scope:"/"`, `display:"standalone"`, `background_color`/`theme_color:"#1e293b"`,
`icons` (192 и 512 + один `purpose:"maskable"`). В v1 имя статическое «onebase»
(динамика из `Cfg.AppName` через шаблон — отдельная мелкая итерация при желании).

**Иконки**: сгенерировать `icon-192.png` и `icon-512.png` (из текущего символа ⚡ /
логотипа). При нежелании держать бинарники — допустим `icon.svg` (Chrome
принимает), но под Android/iOS надёжнее PNG + maskable.

**`sw.js` (консервативный):**
- `install`: precache оболочки — `/offline.html`, ключевые `/vendor/echarts/*`,
  иконки. `activate`: чистка старых версий кэша (версионируется константой).
- `fetch`:
  - `/vendor/*`, `/icons/*`, manifest → **cache-first**.
  - навигация (`/ui/*`, HTML) → **network-first**, фолбэк на `/offline.html`.
    **HTML не кэшируем** (под авторизацией и с живыми данными — иначе риск показать
    устаревшую/чужую страницу).
  - POST и прочее → не перехватываем (network-only).

### Критичные файлы

- `internal/ui/templates.go` — `tplHead` (мета, viewport, PWA-линки, мобильный
  `@media`, регистрация SW), `tplNav` (гамбургер + drawer JS). Основной объём.
- `internal/ui/templates_managed.go` — `@media` для управляемых форм/ТЧ.
- `internal/ui/static.go` — новые маршруты manifest/sw/icons/offline (образец
  стр. 16-19).
- `internal/ui/pwa.go` (новый) — `embed.FS` + хендлеры (образец
  `internal/webassets/assets.go`).
- `internal/ui/pwa/` (новый каталог) — `manifest.webmanifest`, `sw.js`,
  `offline.html`, `icons/icon-192.png`, `icons/icon-512.png`.

## Тесты

- Юнит: маршруты PWA отдают корректные `Content-Type` и заголовки кэширования
  (`/manifest.webmanifest`, `/sw.js` с `no-cache`, `/icons/*` immutable) —
  по образцу существующих тестов хендлеров `internal/ui`.
- Юнит: `manifest.webmanifest` валиден как JSON и содержит обязательные поля
  (`name`, `start_url`, `display`, `icons`).
- Smoke: рендер `tplHead` содержит `viewport` и `<link rel="manifest">`.

## Verification

1. **Сборка**: `go build ./...`, `go vet ./internal/ui/...`
   (go.exe: `C:\Users\i.titov\go-sdk\go\bin`, не в PATH).
2. **Десктоп не сломан** (главная проверка): открыть в десктопном браузере на
   широком окне — вёрстка идентична прежней, гамбургер скрыт, сайдбар на месте.
   Сравнить главную/список/форму/отчёт до и после.
3. **Мобайл-эмуляция**: Chrome DevTools → Device toolbar (iPhone/Pixel) — сайдбар
   уезжает в drawer, гамбургер открывает/закрывает его и оверлей, таблицы
   скроллятся по горизонтали, формы в одну колонку, кнопки крупные.
4. **PWA-аудит**: DevTools → Application → Manifest (иконки/имя/`display`),
   Service Workers (активен, scope `/`); Lighthouse → PWA → «installable».
5. **Реальный телефон**: `https://demo.ivantitov.tech/ui/` → «Добавить на главный
   экран» → запуск полноэкранно; проверить ввод в форму, список, дашборд/отчёт.
6. **SW-обновления**: `/sw.js` отдаётся с `no-cache`; при смене версии кэша старый
   кэш вычищается в `activate`.

## Эстимейт

≈ 3–4 дня:
- viewport + мобильный CSS + гамбургер-drawer (templates.go) — 1.5 дня.
- адаптив управляемых форм и таблиц (templates_managed.go) — 0.5 дня.
- PWA-ассеты (manifest, sw.js, offline, иконки) + маршруты + тесты — 1 день.
- проверка на устройствах, Lighthouse, правки — 0.5–1 день.

## Доработки по ревью (PR #34)

При мерже в `main` применены правки по итогам код-ревью:

- **PWA-ассеты публичны.** `mountPWA` вынесен из auth-группы в публичную секцию
  `internal/api/server.go` (новый `ui.Server.MountPWA`). Под авторизацией manifest
  (его браузер фечит без credentials) и иконки отдавали бы 401 → PWA не
  устанавливался на инстансе с пользователями. Тест
  `internal/api/pwa_public_test.go`.
- **Авто-версия кэша SW.** Имя кэша = `onebase-<vcs.revision>` (подстановка в
  `__OB_CACHE__` при отдаче `/sw.js`), каждый релиз авто-инвалидирует старый кэш —
  vendor-ассеты с неверсионируемыми URL больше не залипают, ручной bump не нужен.
- **Устойчивость SW.** Precache поэлементно (`Promise.allSettled`), кэшируем только
  `res.ok`, navigate-фолбэк через `Response.error()` (нет `respondWith(undefined)`).
- **Сужен скролл таблиц.** Верстальные key/value-таблицы помечены `.tbl-plain` и не
  схлопываются; широкие гриды по-прежнему скроллятся.
- **A11y drawer.** `aria-controls`/`aria-expanded` на ☰, закрытие по `Esc`.
- **manifest `id`.** Стабильная идентичность установки.
