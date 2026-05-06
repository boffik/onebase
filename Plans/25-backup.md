# Этап 25 — Бэкап и восстановление

**Статус:** ✅ Реализовано

## Контекст

Пользователь, который ведёт реальный учёт, должен быть уверен, что данные не потеряются. Кнопка «Бэкап» в лаунчере — психологическая безопасность, без которой производственное использование невозможно.

## UX

### Из лаунчера

- Кнопка **«Бэкап»** рядом с каждой базой → выбор папки → создаёт `backup_ИмяБазы_2026-05-06_14-30.sql.gz`
- Кнопка **«Восстановить»** → выбор файла `.sql.gz` → подтверждение → восстановление

### Из CLI

```bash
# Создать бэкап
onebase backup --db "postgres://localhost/sklad" --out ./backups/

# Восстановить из файла
onebase restore --db "postgres://localhost/sklad" --file ./backups/backup_sklad_2026-05-06.sql.gz

# Автоматический бэкап по расписанию
onebase backup --db "..." --out ./backups/ --schedule "0 2 * * *"   # каждую ночь в 02:00
```

### Автоматические бэкапы

В `config/app.yaml` или в Администрировании → Регламентные задания:
```yaml
backup:
  enabled: true
  schedule: "0 2 * * *"
  keep_last: 7          # хранить последние N бэкапов
  directory: ~/.onebase/backups/<base-id>/
```

## Реализация

**Бэкап:**
- `pg_dump -Fc` (custom format, сжатый) через `os/exec`
- Если `pg_dump` недоступен — fallback на `COPY TO STDOUT` через Go + gzip
- Включает: таблицы данных + конфигурацию + пользователей + регламентные задания

**Восстановление:**
- `pg_restore` или построчный SQL через `pgx`
- Предупреждение если целевая БД не пустая

**Новый файл:** `internal/backup/backup.go`
- `func Dump(ctx, connStr, outPath string) error`
- `func Restore(ctx, connStr, filePath string) error`
- `func Schedule(cfg BackupConfig, sched *scheduler.Scheduler)`

**`internal/cli/`:**
- Новые команды `onebase backup` и `onebase restore`

**`internal/launcher/`:**
- Кнопки «Бэкап» / «Восстановить» в UI лаунчера

## Тесты

- `Dump → Restore` round-trip: создать запись, сделать бэкап, очистить БД, восстановить, проверить запись
- Автобэкап: мок планировщика регистрирует задание

## Эстимейт

3–4 дня.
