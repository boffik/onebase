# Plan: Поддержка SQLite — файловая база данных (аналог 1С)

## Контекст

Цель: дать пользователю выбор при создании базы — **«файловая»** (SQLite, один файл, без PostgreSQL) или **«серверная»** (PostgreSQL, как сейчас). Для pet-проектов файловая база идеальна: распаковал, запустил, работает.

Архитектура: SQLite не открывается напрямую клиентами. OneBase HTTP-сервер — единственный процесс, работающий с файлом. Пользователи подключаются через браузер по HTTP. Это даёт многопользовательский доступ и публикацию в веб, как файловая база 1С.

**Минимальная версия SQLite: 3.45+** (для нативной поддержки JSONB и `RETURNING`). Это релиз января 2024 — для `modernc.org/sqlite` свежие сборки уже включают её.

## Текущее состояние (что мешает)

| Проблема | Фактический масштаб | Где |
|----------|---------|-----|
| `pgxpool.Pool` хранится в 4 файлах, остальные 93 «упоминания» — это сигнатуры/комментарии | 4 файла | `auth/users.go`, `configdb/repo.go`, `launcher/cfgauth.go`, `storage/pg.go` |
| `pgx.Tx`/`pgx.Rows`/`pgx.Row` в публичных API | 8 точек | `internal/storage/tx.go`, `internal/dsl/interpreter/transactions.go` |
| `$N` placeholders | ~60 мест | `internal/storage/*.go` + `internal/auth/roles.go` |
| PostgreSQL-типы DDL: `TIMESTAMPTZ`, `UUID`, `NUMERIC`, `JSONB`, `BOOLEAN` | DDL-генератор | `internal/storage/ddl.go` |
| `information_schema.columns` для проверки колонок | миграции | `internal/storage/migrate.go` |
| `pg_database` + `CREATE DATABASE` | `EnsureDatabase` | `internal/storage/pg.go` |
| `gen_random_uuid()`, `now()` | SQL-функции | в queries |
| `JSONB` + касты `::jsonb` | 8 мест | `auth/roles.go`, `storage/audit.go`, `storage/constants.go` |
| `::text`, `::numeric` касты | 6+ мест | `storage/crud.go`, `storage/journal.go`, `query/query.go` |
| `ILIKE` | `audit.go:146` | `internal/storage/audit.go` |
| `RETURNING` | 6+ мест | `auth/roles.go`, `attachments.go`, `numerator.go`, `seq.go`, `scheduled.go` |
| **`DISTINCT ON`** в виртуальных таблицах регистров (СрезПоследних/СрезПервых) | 6 мест | `internal/query/query.go:816,870` |
| `pg_dump` для бэкапов | бэкап-логика | `internal/launcher/backup_handlers.go` |

## Стратегия

**Не переписывать всё сразу.** Слой абстракции, поддерживающий оба движка через интерфейс `storage.DB` + объект `Dialect`.

```
              storage.DB (interface)
              /                \
    storage/pgdb.go         storage/sqlitedb.go
    (PostgreSQL via pgx)    (SQLite via database/sql)
         |                       |
    pgxpool.Pool            modernc.org/sqlite
```

`storage.DB` становится интерфейсом с методами `Exec`, `Query`, `QueryRow`, `BeginTx`. Каждый драйвер реализует этот интерфейс. SQL генерируется через `Dialect`, который знает специфику каждого движка.

## Этапы

### Этап 1: Абстракция `storage.DB` (4-5 дней)

**Цель**: Заменить конкретный `*pgxpool.Pool` на интерфейс, чтобы можно было подставить SQLite.

**Файлы**: `internal/storage/`, `internal/auth/`, `internal/configdb/`, `internal/launcher/cfgauth.go`, `internal/dsl/interpreter/transactions.go`.

1. Создать `storage/interface.go`:
```go
type DB interface {
    Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) Row
    BeginTx(ctx context.Context) (Tx, context.Context, error)
    Dialect() Dialect
    FilesDir() string
    Close()
}

type Tx interface {
    Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) Row
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}

type Rows interface { Next() bool; Scan(dst ...any) error; Close() error; Err() error; ... }
type Row  interface { Scan(dst ...any) error }
type CommandTag struct { RowsAffected int64 }
```

2. Перенести текущий `*pgxpool.Pool` в `storage/pgdb.go`, реализовать `storage.DB` поверх.
3. Заменить `pgx.Tx`/`pgx.Rows`/`pgx.Row` в `tx.go` и `transactions.go` на свои интерфейсы (они и так почти совпадают по сигнатурам).
4. Обновить публичные приёмники: `auth.Repo`, `configdb.Repo`, `cfgauth` — принимают `storage.DB`, не `*pgxpool.Pool`.

**Проверка**: всё работает как раньше, тесты зелёные, но через интерфейс.

### Этап 2: Диалект SQL (5-6 дней)

**Цель**: Единая генерация SQL, корректная для обоих движков. **Это самый сложный этап** — здесь сидит `DISTINCT ON`.

1. Создать `storage/dialect.go`:
```go
type Dialect interface {
    // Placeholders
    Placeholder(n int) string                       // "$N" или "?"

    // Functions
    Now() string                                    // "now()" или "datetime('now')"
    NewUUID() string                                // "" — генерим в Go для обоих
    LowerLike(col string) string                    // "LOWER(col::text)" или "LOWER(CAST(col AS TEXT))"
    CaseInsensitiveLike() string                    // "ILIKE" или "LIKE ... COLLATE NOCASE"

    // Types
    TypeBool() string                               // "BOOLEAN" или "INTEGER"
    TypeText() string
    TypeNumber(precision, scale int) string         // "NUMERIC(p,s)" или "REAL"
    TypeTimestamp() string                          // "TIMESTAMPTZ" или "TEXT"
    TypeUUID() string                               // "UUID" или "TEXT"
    TypeJSON() string                               // "JSONB" или "BLOB" (SQLite JSONB) или "TEXT"
    JSONCast() string                               // "::jsonb" или "" (пусто для SQLite)

    // DDL helpers
    AddColumnSQL(table, col, typ string) string     // "ALTER ... IF NOT EXISTS" vs PRAGMA-based
    ColumnExists(ctx, db, table, col) (bool, error) // information_schema vs PRAGMA table_info
    CreateDatabase(ctx, dsn, name) error            // CREATE DATABASE vs touch file

    // Window function: срез последних/первых из регистра сведений
    LatestPerKey(cols, partitionBy, orderBy []string, table, where string) string
}
```

2. Реализовать `PgDialect` и `SQLiteDialect`.
3. **Виртуальные таблицы (`internal/query/query.go:816,870`)** — переписать `DISTINCT ON` на единый запрос через `LatestPerKey`. Для PG он генерирует `DISTINCT ON`, для SQLite — `ROW_NUMBER() OVER (PARTITION BY ... ORDER BY ...) WHERE rn = 1`. Это даст одинаковую семантику на обоих движках.
4. В `ddl.go` — замена литералов типов на `dialect.TypeXxx()`.
5. В `migrate.go` — замена `information_schema` на `dialect.ColumnExists()`.
6. В `crud.go`/`journal.go`/`query.go` — замена `LOWER(col::text) LIKE LOWER($N)` на `dialect.LowerLike(col) + dialect.Placeholder(n)`.
7. В `audit.go` — замена `ILIKE $N` на единый вызов диалекта.
8. **JSONB-стратегия**: для SQLite используем нативный `JSONB` (BLOB) с функциями `json()`, `json_extract()`, `jsonb()`. Это требует SQLite ≥ 3.45. `::jsonb` касты убираются (для PG диалект возвращает `::jsonb`, для SQLite пусто, а серверная функция `jsonb()` оборачивает вход).
9. **`RETURNING`** — поддерживается обоими движками (PG всегда, SQLite ≥3.35). Оставляем как есть, фиксируем минимальную версию.

**Ключевые различия SQL:**

| Конструкция | PostgreSQL | SQLite |
|---|---|---|
| Параметры | `$1`, `$2` | `?` |
| UUID | `UUID` + `gen_random_uuid()` | `TEXT`, генерация в Go (`google/uuid`) |
| Дата/время | `TIMESTAMPTZ` | `TEXT` ISO 8601 + парсинг в Go |
| Числа | `NUMERIC(p,s)` | `REAL` |
| UPSERT | `ON CONFLICT DO UPDATE` | `ON CONFLICT DO UPDATE` (≥3.24) |
| `RETURNING` | да | да (≥3.35) |
| `ADD COLUMN IF NOT EXISTS` | да | нет — `PRAGMA table_info` + ALTER |
| Проверка колонок | `information_schema.columns` | `PRAGMA table_info(table)` |
| Boolean | `BOOLEAN` | `INTEGER` (0/1) |
| `now()` | `now()` | `datetime('now')` |
| `ILIKE` | да | `LIKE ... COLLATE NOCASE` или `LOWER(...)` |
| Касты `::text`, `::numeric` | да | `CAST(x AS TEXT)`, `CAST(x AS REAL)` |
| **`DISTINCT ON`** | **да** | **нет — переписываем на `ROW_NUMBER() OVER`** |
| `JSONB` | `JSONB` + `::jsonb` | `BLOB` + `jsonb()` (≥3.45) |
| Создание БД | `CREATE DATABASE` | создание файла |

**Проверка**: тесты `storage_test`, `query_test`, включая `virtualtables_test.go`, проходят на обоих диалектах.

### Этап 3: Драйвер SQLite (3-4 дня)

**Цель**: Реализация `storage.DB` для SQLite.

1. Зависимость: `modernc.org/sqlite` (pure Go, **без CGO**).
2. `storage/sqlitedb.go`:
   - `ConnectSQLite(ctx, filePath)` — открывает файл, выполняет `PRAGMA`.
   - Адаптер `database/sql` ↔ интерфейс `storage.DB`.
3. Обязательные PRAGMA при открытии:
   - `journal_mode=WAL` — конкурентное чтение при пишущей транзакции.
   - `synchronous=NORMAL` — баланс надёжности и скорости.
   - `foreign_keys=ON` — иначе FK игнорируются.
   - `busy_timeout=5000` — ожидание при конкурентной записи.
   - `cache_size=-64000` — 64 МБ кеша.
4. UUID — через `github.com/google/uuid`, хранение `TEXT(36)`.
5. Даты — `time.Time` ↔ ISO 8601 `2006-01-02T15:04:05.999999Z07:00`. Конвертация в адаптере `Scan`/`Exec`.
6. Транзакции — `database/sql.Tx`, изоляция по умолчанию `DEFERRED`. Помнить: SQLite сериализует записи (одна пишущая транзакция на БД).

**Проверка**: новый unit-тест `sqlitedb_test.go` — CRUD, транзакции, конкурентное чтение в WAL.

### Этап 4: Бэкап и восстановление для SQLite (1-2 дня)

**Цель**: Бэкап не через `pg_dump`, а нативно.

1. SQLite бэкап: `VACUUM INTO 'backup_file.db'` — атомарный online-бэкап без блокировки записи.
2. Восстановление: остановить активные сессии БД → закрыть пул → заменить файл → переоткрыть.
3. В `backup_handlers.go` — роутинг по `base.DBType`.

**Проверка**: бэкап/восстановление обоих типов через UI.

### Этап 5: UI выбора типа базы (2 дня)

**Цель**: UX выбора в лаунчере.

1. Расширить `Base`:
```go
type Base struct {
    ...
    DBType string `yaml:"db_type"` // "postgres" | "sqlite", default "postgres"
    DBPath string `yaml:"db_path,omitempty"` // для sqlite — путь к .db файлу
}
```

2. UI лаунчера при добавлении базы:
   - Переключатель «Файловая / Серверная».
   - Файловая: автогенерация пути `<onebase_dir>/databases/<name>.db` или выбор пользователем.
   - Серверная: DSN PostgreSQL (как сейчас).
3. `storage.Connect()` — роутинг:
```go
func Connect(ctx context.Context, base *Base) (DB, error) {
    switch base.DBType {
    case "sqlite": return ConnectSQLite(ctx, base.DBPath)
    case "", "postgres": return ConnectPostgres(ctx, base.DB)
    default: return nil, fmt.Errorf("unknown db_type: %s", base.DBType)
    }
}
```
4. Совместимость: пустой `DBType` = `postgres` (старые конфиги работают).

**Проверка**: создание базы обоих типов через UI, обе открываются, миграции прогоняются.

### Этап 6: JSONB-точки и аудит (2-3 дня)

**Цель**: Привести 8 JSONB-мест к диалекту.

1. `auth/roles.go:99-100` — `permissions JSONB` → `dialect.TypeJSON()`, касты `$4::jsonb` → `dialect.JSONCast()`.
2. `storage/audit.go:71-72,115` — `old_value`/`new_value` JSONB + `$8::jsonb, $9::jsonb`.
3. `storage/constants.go:13` — `value JSONB`.
4. Запросы по JSON — через диалект:
   - PG: `permissions->'foo'`.
   - SQLite: `json_extract(permissions, '$.foo')`.
   Завернуть в helper `dialect.JSONField(col, path string)`.

**Проверка**: roles, audit, constants работают на обоих движках; читаются/пишутся одинаковые данные.

### Этап 7: Тестирование и полировка (5-7 дней)

**Цель**: Регрессии и финальная сборка.

1. **Test matrix**: добавить флаг в `go test` для прогона того же набора на обоих движках. Использовать build tag или env var (`ONEBASE_TEST_DB=sqlite`).
2. Прогнать на SQLite:
   - `internal/storage/*_test.go` — CRUD, миграции, журналы, audit, constants.
   - `internal/query/*_test.go` — включая `virtualtables_test.go` (срез последних/первых).
   - `internal/auth/*_test.go` — roles, permissions.
   - `internal/dsl/interpreter/*_test.go` — транзакции DSL.
3. Интеграционный smoke-тест: создать SQLite-базу, провести документ, отчёт по регистру, бэкап, восстановление.
4. Бенчмарки: PG vs SQLite на типовых операциях (insert документа, отчёт по регистру).
5. Сборка: `CGO_ENABLED=0 go build` — должна работать (modernc.org/sqlite pure Go).
6. Документация: README и `DEVELOPER.md` — раздел про выбор движка, лимиты SQLite, миграцию данных.

**Проверка**: CI зелёный на обоих движках, smoke-тест проходит.

## Что НЕ делаем (явный скоуп)

- **Миграция данных между PG↔SQLite** — отдельная фича, не в этом плане. Пользователь выбирает тип при создании, переключение требует ручного экспорта/импорта (можно добавить позже).
- **Полнотекстовый поиск (FTS)** — если будет нужен, в SQLite это FTS5, в PG это tsvector. Сейчас не используется.
- **Шифрование БД** — SQLCipher требует CGO. Откладываем.
- **Репликация / multi-writer** — у SQLite один писатель, для pet-проектов это норма.

## Итого по времени

| Этап | Дни | Зависимость |
|------|-----|-------------|
| 1. Абстракция DB | 4-5 | — |
| 2. Диалект SQL (включая `DISTINCT ON` → window functions) | 5-6 | Этап 1 |
| 3. Драйвер SQLite | 3-4 | Этап 1, 2 |
| 4. Бэкап для SQLite | 1-2 | Этап 3 |
| 5. UI выбора типа | 2 | Этап 3 |
| 6. JSONB и аудит | 2-3 | Этап 2, 3 |
| 7. Тестирование | 5-7 | Все |
| **Итого** | **~22-29 дней** | |

## Риски

1. **`DISTINCT ON` → window function** — семантика близка, но не всегда тождественна. Нужна сверка результата на ненулевой выборке регистров. Если есть тест-набор виртуальных таблиц — прогнать на обоих движках с одинаковыми данными.
2. **Сериализация записи в SQLite** — для pet-проекта норма, но при активной параллельной нагрузке (бэкграунд-задания + ручной ввод) могут быть `SQLITE_BUSY`. WAL + `busy_timeout=5000` это смягчают, но не убирают полностью.
3. **Точность чисел**: PG `NUMERIC(p,s)` — точные десятичные, SQLite `REAL` — IEEE 754 float. **Для денежных сумм это потенциальная проблема.** Решение: хранить деньги как `INTEGER` (копейки) или `TEXT` с конвертацией в Go через `shopspring/decimal`. Решить **до Этапа 2**, влияет на схему.
4. **Даты как TEXT** — медленнее сравнений, чем `TIMESTAMPTZ`. Для pet-проектов незначительно, но индекс по дате будет менее эффективен.
5. **Версия SQLite** — `modernc.org/sqlite` тянет встроенную версию. Проверить, что собранная версия ≥3.45 (для JSONB). Если нет — фоллбек на `TEXT` для JSON.
6. **Бэкап во время записи** — `VACUUM INTO` атомарен, но требует свободного места на диск. Документировать.

## Что НЕ меняется

- DSL-интерпретатор (работает поверх `storage.DB`/`storage.Tx`).
- UI (браузерный, не зависит от БД).
- Конфигуратор (метаданные в YAML).
- Print forms, макеты, отладчик.
- API запросов и query builder (генерация SQL уже идёт через единый pipeline, диалект подключается там).
