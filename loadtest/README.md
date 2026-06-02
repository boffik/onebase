# Нагрузочное тестирование onebase

Слой HTTP-нагрузки (k6) нагрузочной стратегии onebase. Дополняет два других слоя,
живущих в основном коде:

- **Go-микробенчмарки** (`internal/.../*_bench_test.go`) — регрессии в CI;
  `go test -bench . -run NONE ./internal/dsl/interpreter/ ./internal/storage/ ./internal/entityservice/`.
- **`onebase bench`** — синтетический балл в стиле Gilev (док/сек + APDEX),
  ходит во внутренние пакеты мимо HTTP; `onebase bench --help`.
- **k6 (эта папка)** — реалистичная HTTP-нагрузка на REST API: поиск узких мест
  и сайзинг.

## Что внутри

```
loadtest/
├── Dockerfile            # образ onebase для стенда (контекст сборки — корень репо)
├── docker-compose.yml    # PostgreSQL + onebase + Prometheus + Grafana + k6
├── prometheus.yml        # scrape-конфиг (/metrics закрыт токеном — шлём ?token=)
├── seed/main.go          # Go-сидер: наполняет базу через REST, пишет id в JSON
└── k6/
    ├── lib/common.js                 # BASE_URL, токен, хелперы (createCounterparty, postReceipt)
    └── scenarios/
        ├── post_document.js          # ГЛАВНЫЙ: создание+проведение документа (OnPost + движения)
        ├── catalog_crud.js           # лёгкий путь: список + создание справочника
        └── list_query.js             # read-heavy: списки с сортировкой
```

Эталонная конфигурация — `examples/simple-erp` (сущности **Контрагент** и
**Поступление**). Под другой конфиг поменяйте имена сущностей/полей в
`k6/lib/common.js` и `seed/main.go`.

## Быстрый старт (Docker)

```bash
# 1. Поднять стенд (PostgreSQL + onebase + Prometheus + Grafana)
docker compose -f loadtest/docker-compose.yml up -d --build

# 2. Засеять данные (с хоста; пишет loadtest/seed/counterparties.json)
go run ./loadtest/seed -url http://localhost:8080 -counterparties 200 -documents 500 \
  -out loadtest/seed/counterparties.json

# 3. Прогнать главный сценарий (проведение документов)
docker compose -f loadtest/docker-compose.yml run --rm k6 run /scripts/scenarios/post_document.js

# Метрики: Prometheus http://localhost:9090 · Grafana http://localhost:3000 (admin/admin)
```

Остановить и удалить: `docker compose -f loadtest/docker-compose.yml down -v`.

## Запуск без Docker

Нужен установленный [k6](https://k6.io) и запущенный onebase с включёнными
метриками/pprof:

```bash
ONEBASE_DEBUG_TOKEN=loadtest onebase migrate --project examples/simple-erp --db "$DATABASE_URL"
ONEBASE_DEBUG_TOKEN=loadtest onebase run     --project examples/simple-erp --db "$DATABASE_URL" --port 8080

go run ./loadtest/seed -url http://localhost:8080 -out loadtest/seed/counterparties.json

k6 run -e BASE_URL=http://localhost:8080 \
       -e SEED_FILE=../../seed/counterparties.json \
       loadtest/k6/scenarios/post_document.js
```

## Аутентификация

Проще всего гонять нагрузку по базе **без пользователей** — onebase пускает
анонимно. Если в базе есть юзеры, получите сессионный токен и передайте его k6
через `-e OB_TK=<token>` (добавляется к каждому запросу как `?_tk=…`). Сидер
умеет логиниться флагами `-login/-password`.

## Профили нагрузки и пороги

- `post_document.js` — ramping-vus 0→20→50, порог `p95<800мс`, ошибок `<1%`.
- `catalog_crud.js` — постоянные 30 VU, 70% чтений / 30% записей, `p95<300мс`.
- `list_query.js` — ramping-arrival-rate до 200 rps, `p95<500мс`, `p99<1500мс`.

Пороги в `options.thresholds` каждого файла — правьте под свои SLA. Для сайзинга
поднимайте `target`/`stages` ступенями и смотрите, где p95 пробивает SLA.

## Поиск узких мест

Во время прогона снимайте профили и метрики (токен `ONEBASE_DEBUG_TOKEN`):

```bash
# CPU-профиль на 30 секунд под нагрузкой
go tool pprof "http://localhost:8080/debug/pprof/profile?token=loadtest&seconds=30"
# Куча
go tool pprof "http://localhost:8080/debug/pprof/heap?token=loadtest"
```

В Prometheus/Grafana смотрите `onebase_http_request_duration_seconds` (латентность
по маршрутам) и `onebase_db_pool_*` (насыщение пула соединений pgx) — это
разделяет «упёрлись в интерпретатор/CPU» и «упёрлись в БД/пул».
