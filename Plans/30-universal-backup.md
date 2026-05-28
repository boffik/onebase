# План 30: Кросс-энжинный формат полного бэкапа (PostgreSQL ↔ SQLite)

**Статус**: Спроектировано, реализация впереди
**Приоритет**: Высокий — блокирует ключевой 1С-сценарий «перенести базу с сервера на ноутбук»
**Аналог**: формат `.dt` в 1С:Предприятии (engine-neutral дамп базы)

## Context

Сейчас полный бэкап `.obz` сохраняет внутри **бинарный** дамп: `database.sql.gz` (для PostgreSQL через `pg_dump`) или `database.db` (для SQLite через `VACUUM INTO`). При попытке восстановить такой `.obz` в базу другого типа платформа отказывает в `internal/launcher/backup_handlers.go:86-97` (`checkBackupFileMismatch`) — формат несовместим. Пользователь получает ошибку «Нельзя восстановить PostgreSQL-бэкап в SQLite-базу».

Это блокирует ключевой 1С-сценарий: «перенести базу с сервера на ноутбук» (файловый режим SQLite) и наоборот — аналог `.dt` файла в 1С:Предприятии, который технологически нейтрален.

Цель — добавить **универсальный** формат внутри того же `.obz`-контейнера, который переносит и схему, и данные, и системные таблицы, и файлы вложений в engine-нейтральном виде. Целевой движок определяется по DSN базы-получателя, DDL генерируется через уже существующий `Dialect`-интерфейс, данные пишутся через диалект-нейтральные `db.Migrate` + `db.Upsert`. Старый бинарный формат сохраняется как опция «быстрый бэкап» для случая того же движка.

Ожидаемый результат: пользователь создаёт PG-базу `trade`, нажимает «Создать полный бэкап» (галка «Совместимый формат» включена) → получает `trade.obz`. Создаёт пустую SQLite-базу `trade-local`, загружает в неё `.obz` → все справочники, документы, движения регистров, пользователи, константы, вложения переезжают без вмешательства. Обратное направление — симметрично.

## Объём

Выбрано: **максимум данных + вложения** (полный аналог `.dt`), **чекбокс «Совместимый формат»** в UI создания бэкапа (по умолчанию включён).

В универсальный архив попадает:
- Все прикладные таблицы (catalogs, documents + табчасти, registers, inforegs, accountregs).
- Системные таблицы: `_users`, `_onebase_config`, `_attachments`, `_constants`, `_numerators`, `_audit`, `_scheduled_runs`, `_predefined_values`.
- Файлы вложений из `~/.onebase/files/<база>/`.
- Конфигурация (YAML) — в существующем разделе `config/`.

## Структура нового `.obz` v2

ZIP с расширением `META.txt::format=universal`. Старые `.obz` (`format` отсутствует или `format=binary`) читаются как раньше — детект формата по `META.txt`.

```
backup.obz                     ZIP-контейнер
├── META.txt
│   ├── version=2
│   ├── format=universal       ← новый ключ (legacy: отсутствует / "binary")
│   ├── source_db_type=postgres|sqlite
│   ├── source_base=Trade
│   ├── date=2026-05-15T18:30:00Z
│   ├── platform_version=1.0.0
│   └── has_attachments=true|false
├── manifest.json              сводка сущностей и счётчики строк
├── config/                    YAML-конфигурация (как сейчас)
│   ├── app.yaml
│   ├── catalogs/…
│   ├── documents/…
│   └── …
├── data/                      прикладные таблицы — по одной JSONL на сущность
│   ├── catalogs/Номенклатура.jsonl
│   ├── documents/Реализация.jsonl
│   ├── tableparts/Реализация_Товары.jsonl
│   ├── registers/ОстаткиТоваров.jsonl
│   └── inforegs/ЦеныНоменклатуры.jsonl
├── system/                    системные таблицы — JSONL
│   ├── _users.jsonl           bcrypt-хэши паролей сохраняются как есть
│   ├── _attachments.jsonl     метаданные файлов (id, owner, filename, mime, size, uploaded_*)
│   ├── _constants.jsonl
│   ├── _numerators.jsonl      текущие счётчики — иначе ПРД-00042 после restore станет ПРД-00001
│   ├── _audit.jsonl
│   ├── _scheduled_runs.jsonl
│   └── _predefined_values.jsonl
└── attachments/               бинарные файлы по UUID (плоско)
    ├── 550e8400-e29b-41d3-a716-446655440001
    └── 550e8400-e29b-41d3-a716-446655440002
```

### Нормализация типов в JSONL

Чтобы один и тот же JSON корректно импортировался и в PostgreSQL, и в SQLite:

| Тип             | JSON                                | Импорт                                   |
|-----------------|-------------------------------------|------------------------------------------|
| `uuid.UUID`     | `"550e8400-..."` (string)           | Cast в `Dialect.TypeUUID()` колонку      |
| Numeric/Decimal | `"1234.56"` (string, без потери)    | Парсится Upsert'ом в `pgtype.Numeric` / string в SQLite |
| Timestamp       | `"2026-05-15T18:30:00Z"` (RFC 3339) | `TIMESTAMPTZ` или TEXT через Dialect     |
| bool            | `true` / `false`                    | `BOOLEAN` / `INTEGER` 0/1                |
| BYTEA / BLOB    | `"<base64>"`                        | Декод обратно в `[]byte`                 |
| JSONB           | вложенный объект                    | `Dialect.JSONCast()`                     |
| NULL            | `null`                              | NULL                                     |

Решение «Numeric как string» защищает от потери precision при больших суммах (1С-учётные данные могут быть с 4 знаками после запятой и 12 знаками целой части).

## Изменения по файлам

### 1. `internal/backup/universal.go` — новый файл

Две функции верхнего уровня:

```go
// ExportUniversal пишет .obz формата v2/universal в w.
// project — текущая загруженная конфигурация (для перечисления сущностей).
// attachmentsDir — путь ~/.onebase/files/<base>/.
func ExportUniversal(
    ctx context.Context,
    db *storage.DB,
    project *metadata.Project,
    attachmentsDir string,
    w io.Writer,
) error

// ImportUniversal читает .obz формата v2/universal из r и наливает данные
// в подключённую целевую базу (любого движка).
// cfgRepo используется для загрузки конфигурации из config/.
func ImportUniversal(
    ctx context.Context,
    db *storage.DB,
    cfgRepo *configdb.Repo,
    attachmentsDir string,
    r io.ReaderAt,
    size int64,
) (*ImportReport, error)
```

Внутри Export:
1. Записать `META.txt` с `format=universal`.
2. Слить конфигурацию в `config/` (если `source=database` → выгрузить из `_onebase_config`, иначе скопировать YAML с диска).
3. Для каждой сущности из `project` (catalogs, documents, tableparts, registers, inforegs, accountregs) — `db.QueryAll(ctx, "SELECT * FROM "+quote(table), nil)` → нормализовать каждое значение → `json.Marshal` построчно в `data/<kind>/<name>.jsonl`. Использовать поток (`zip.Writer` + `bufio.Writer`), не материализовать в память.
4. Системные таблицы — список фиксированный, читается тем же `QueryAll`. Каждая в свой `system/<name>.jsonl`.
5. Если `attachmentsDir` существует — обойти `filepath.Walk` и положить каждый файл в архив под `attachments/<uuid>`. Имена файлов в `_attachments.filename` сохранены в JSONL.
6. В конце — записать `manifest.json` со списком всех записанных файлов и счётчиками (count rows per entity), для верификации при импорте.

Внутри Import:
1. Распарсить `META.txt`, проверить `format=universal` и `version=2`. Если `format` отсутствует — вернуть `ErrLegacyFormat` (caller перенаправит на старую ветку).
2. Загрузить `manifest.json`.
3. Загрузить `config/` в целевую базу через `cfgRepo.ImportFromDir()` (имеющаяся функция).
4. Запустить полный цикл создания схемы (зеркало `onebase deploy`): `db.Migrate()`, `MigrateRegisters()`, `MigrateInfoRegisters()`, `MigrateConstants()`, `SyncAccounts()`, `MigrateAccountRegisters()`, `EnsureAuditSchema()`, `EnsureScheduledRunsTable()`, `EnsureAttachmentTable()`, `authRepo.EnsureSchema()`. DDL генерируется через `db.Dialect()` — автоматически правильный для целевого движка.
5. Транзакционно (одна big TX): `DELETE FROM` для каждой сущности и системной таблицы, затем построчно `INSERT` через диалект-нейтральные prepared statements. Использовать `db.Upsert()` для прикладных сущностей; для системных таблиц — прямой `INSERT INTO _users (...) VALUES (...)` с заполнением плейсхолдеров через `Dialect.Placeholder(i)`.
6. Соблюдать порядок: catalogs (с разрешением `parent_id` через двухпроходный insert или `DEFER` constraints) → documents → tableparts → predefined → registers → inforegs → accountregs → _users/_constants/_numerators/_audit/_scheduled_runs/_attachments.
7. Распаковать `attachments/<uuid>` в `attachmentsDir/<owner_name>/<uuid>`, имя owner_name берётся из `_attachments` JSONL.
8. Сверить счётчики строк с `manifest.json` → собрать `ImportReport`.

### 2. `internal/launcher/backup_handlers.go` — ветвление формата

- В `backupFullExport`: читать поле формы `compatible` (новый чекбокс). Если включено или поле отсутствует (для CLI) → `ExportUniversal`. Иначе → существующая бинарная ветка (`pg_dump`/`VACUUM INTO`).
- В `backupFullImport`: первым шагом распаковать только `META.txt`, прочитать `format`. Если `universal` → `ImportUniversal`. Если `binary` или отсутствует → существующая ветка с `checkBackupFileMismatch`.
- `checkBackupFileMismatch` оставить без изменений — она нужна для legacy.

### 3. `internal/cli/backup.go` — CLI

- `onebase backup --full --out DIR [--binary]` — по умолчанию universal; `--binary` для быстрого дампа того же движка.
- `onebase restore --file PATH` — формат определяется автоматически по `META.txt`. Никаких новых флагов.

### 4. UI чекбокса

В шаблоне формы «Создать полный бэкап» (находится в `internal/launcher/backup_handlers.go` рядом с `backupFullExport` — там inline HTML, либо в `configurator_tmpl.go` если форма вынесена) добавить:

```html
<label style="display:flex;gap:6px;align-items:center;margin:8px 0">
  <input type="checkbox" name="compatible" checked>
  <span>Совместимый формат (можно загрузить в базу другого типа: PostgreSQL ↔ SQLite)</span>
</label>
<div style="font-size:11px;color:#64748b;margin-left:24px">
  Без галки — быстрый бинарный дамп, годится только для того же типа БД.
</div>
```

### 5. Тесты — `internal/backup/universal_test.go`

- `TestExportUniversalRoundTripSameEngine` — PG → universal → новая PG. Сравнить count и контрольные ID. На тестовом контейнере (`TEST_DATABASE_URL`).
- `TestExportUniversalCrossEngine` — PG → universal → SQLite (in-memory или temp-file). Сравнить count по каждой сущности и спот-проверить несколько строк.
- `TestExportUniversalAttachments` — записать файл вложения, экспорт, импорт в другой движок, проверить, что файл восстановлен с тем же содержимым и привязан к тому же документу через `_attachments`.
- `TestImportUniversalRejectsLegacy` — попытка распаковать `.obz` без `format=universal` возвращает `ErrLegacyFormat`.
- Юнит-тесты на нормализатор типов: Numeric→string→Numeric round-trip, Timestamp RFC 3339, UUID, NULL, base64 для BYTEA.

## Ключевые файлы

| Файл | Что меняем |
|---|---|
| `internal/backup/universal.go` (новый) | `ExportUniversal` + `ImportUniversal` + нормализатор типов |
| `internal/backup/universal_test.go` (новый) | Round-trip + cross-engine + attachments + reject-legacy |
| `internal/launcher/backup_handlers.go` | Ветвление по `format` в `backupFullExport`/`backupFullImport`; новый чекбокс в форме |
| `internal/cli/backup.go` | Флаг `--binary` (universal по умолчанию); auto-detect при restore |
| `examples/trade/` (опционально) | Эталонная база для теста cross-engine (PG → SQLite) |

## Что переиспользуем

- **`storage.Dialect`** (`internal/storage/dialect.go:11-81`) — автоматическая подстановка типов под движок. Не трогаем.
- **`storage.CreateTableSQL`** (`internal/storage/ddl.go:60`) — DDL под любой движок. Не трогаем.
- **`db.Migrate*`** (`internal/storage/ddl.go` и соседи) — миграция схемы. Вызываем из ImportUniversal.
- **`db.QueryAll`** — engine-нейтральное чтение всех строк (используется в ExportUniversal).
- **`db.Upsert`** (`internal/storage/crud.go:32`) — engine-нейтральная запись. Используется в ImportUniversal.
- **`db.GetByID` + `normalizeValue`** (`internal/storage/crud.go:171`) — уже превращают `pgtype.Numeric`/UUID в Go-типы. Внутри ExportUniversal сделаем свой `marshalValue(v any)` поверх этой логики для JSONL-сериализации.
- **`configdb.Repo.ImportFromDir`** — загрузка YAML в `_onebase_config` (вызывается в ImportUniversal).
- **`authRepo.EnsureSchema`, `EnsureAuditSchema`, `EnsureScheduledRunsTable`, `EnsureAttachmentTable`, `EnsureAccountsTable`** — все вызываем в ImportUniversal перед заливкой данных.
- **`archive/zip` ZIP-инфраструктура** в существующем `backupFullExport` (`backup_handlers.go:284-376`) — копируем подход, добавляем новые директории в архив.
- **`META.txt` парсер** — расширяем существующий новыми ключами `format`, `has_attachments`.
- **«Require stopped base for full restore»** (уже реализовано) — оставляем как есть, ImportUniversal тоже требует остановленную базу.

## Verification

End-to-end на примере `examples/trade`:

1. Собрать: `go build -o onebase.exe ./cmd/onebase`.
2. Развернуть PG-базу: `onebase deploy --project ./examples/trade --db "postgres://localhost/trade_dev?sslmode=disable"`. Запустить, ввести 3-4 документа реализации, прикрепить файл к одному документу.
3. Создать бэкап через UI (с галочкой «Совместимый формат») → получить `trade_2026-05-15.obz`. Размер — проверить, что включены вложения.
4. Распаковать `.obz` вручную (это ZIP) → проверить структуру: `META.txt::format=universal`, `data/catalogs/*.jsonl` с реальными строками, `attachments/<uuid>`.
5. Через лаунчер создать новую базу типа SQLite (`./trade-local.db`).
6. В её конфигураторе → Бэкапы → Восстановить → выбрать `trade_2026-05-15.obz` → дождаться готовности.
7. Запустить SQLite-базу → проверить:
   - Все справочники видны, контрагенты на месте.
   - Все документы реализации видны, проведены, движения регистра «ОстаткиТоваров» совпадают с исходной.
   - Вложение к документу скачивается, контент совпадает (`sha256` бинарный).
   - Текущий номер документа — следующий после последнего из PG (т.е. `_numerators` пережил).
   - Пользователи и пароли работают (логин теми же креденшалами).
8. Обратное направление: SQLite → universal-`.obz` → новая PG → сравнение по аналогии.
9. Регрессия: старый `.obz` (`format=binary` отсутствует) через тот же restore должен работать как раньше, если целевой движок совпадает; и по-прежнему выдавать понятную ошибку при несовпадении (для legacy формата кросс-перенос не поддерживается).
10. `go test ./internal/backup/...` — все тесты зелёные, включая cross-engine.

## Что осознанно НЕ делаем в этой итерации

- Инкрементальный экспорт / WAL-подобный delta-режим — только full snapshot.
- Сжатие per-file внутри ZIP (ZIP-deflate стандартный + JSONL хорошо жмётся; gzip-обвес лишний).
- Параллельный экспорт (goroutine per entity) — последовательно, сначала корректность.
- Поддержка миграции между несовместимыми версиями платформы (`platform_version` в META пока только informational).
- Schema diff / merge при импорте: целевая база перед restore должна быть **пустой** (либо платформа очищает существующие данные через `DELETE FROM`, как уже делает текущий импорт).
- Поддержка третьего движка (MySQL/MSSQL) — за рамками задачи.
