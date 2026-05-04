# Этап 17 — Фоновые / регламентные задания

## Контекст

Автоматическая выгрузка, ночной пересчёт остатков, плановая генерация отчётов, периодические интеграции. Сейчас обработки запускаются только вручную из UI — никакого расписания.

Эта фича добавляет планировщик с cron-выражениями: задания описываются в YAML и автоматически запускаются по расписанию, с журналом запусков и UI для управления.

**Опционально зависит от** этапа 15 (транзакции) — массовые операции в фоне без транзакций рискованны.

---

## YAML

`scheduled/перепроведениеночь.yaml`:

```yaml
name: ПерепроведениеНочь
title: Ночное перепроведение всех документов
schedule: "0 2 * * *"           # cron: каждый день в 2:00
processor: ПерепроведениеВсехДокументов
params:
  ОтДаты: "{{today | minus_days:7}}"
enabled: true
on_error: continue              # continue | stop | retry
timeout: 3600                   # максимум 1 час, потом kill
```

### Примеры расписаний

| schedule | Когда срабатывает |
|---|---|
| `*/5 * * * *` | каждые 5 минут |
| `0 */1 * * *` | каждый час |
| `0 2 * * *` | каждый день в 2:00 |
| `0 9 * * 1-5` | по будням в 9:00 |
| `0 0 1 * *` | 1-го числа каждого месяца |

### Шаблоны параметров

`{{today | minus_days:7}}` — вычисляется в момент запуска. Поддерживаемые трансформы:
- `now`, `today`
- `minus_days:N`, `minus_hours:N`, `minus_months:N`
- `start_of_month`, `end_of_month`

---

## Хранилище

```sql
CREATE TABLE _scheduled_runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name     TEXT NOT NULL,
    started_at   TIMESTAMPTZ NOT NULL,
    finished_at  TIMESTAMPTZ,
    status       TEXT NOT NULL,    -- running | success | error | timeout
    output       TEXT,             -- собрано из Сообщить()
    error        TEXT,
    duration_ms  INTEGER
);
CREATE INDEX idx_scheduled_runs_job ON _scheduled_runs (job_name, started_at DESC);
CREATE INDEX idx_scheduled_runs_at  ON _scheduled_runs (started_at DESC);
```

Состояние enabled/disabled удобно хранить в YAML (синк при старте), либо в `_scheduled_jobs` если нужно менять без перезапуска. На MVP — только в YAML.

---

## Изменения в коде

### `internal/scheduler/` (новый пакет)

```go
package scheduler

import "github.com/robfig/cron/v3"

type Scheduler struct {
    cron     *cron.Cron
    jobs     []*ScheduledJob
    db       *storage.DB
    runtime  *runtime.Service
    log      *slog.Logger
}

type ScheduledJob struct {
    Name      string
    Schedule  string
    Processor string
    Params    map[string]any
    Enabled   bool
    OnError   string  // "continue" | "stop" | "retry"
    Timeout   time.Duration
}

func New(db *storage.DB, runtime *runtime.Service) *Scheduler

// Загружает задания из проекта и регистрирует их в cron.
func (s *Scheduler) LoadFromProject(project *project.Project) error

// Старт планировщика (горутина в фоне).
func (s *Scheduler) Start(ctx context.Context)

// Ручной запуск задания (из UI).
func (s *Scheduler) RunNow(ctx context.Context, jobName string) error

// История запусков для UI.
func (s *Scheduler) Runs(ctx context.Context, jobName string, limit int) ([]Run, error)
```

При срабатывании cron — горутина:
1. INSERT в `_scheduled_runs` со статусом `running`.
2. Резолв шаблонных параметров (`{{today}}`).
3. Запуск обработки через `runtime.RunProcessor(ctx, name, params)`.
4. UPDATE `_scheduled_runs` с финальным статусом, output, error.
5. Audit log через `storage.AuditLog`.

### `internal/cli/dev.go`, `run.go`

Запуск Scheduler в той же горутине что и HTTP-сервер:

```go
sched := scheduler.New(db, runtimeSvc)
if err := sched.LoadFromProject(proj); err != nil { ... }
go sched.Start(ctx)
```

### `internal/metadata/scheduled.go` (новый)

```go
type ScheduledJob struct {
    Name, Title, Schedule, Processor string
    Params  map[string]any
    Enabled bool
    OnError string
    Timeout int  // seconds
}

func LoadScheduledFile(path string) (*ScheduledJob, error)
func LoadScheduledDir(dir string) ([]*ScheduledJob, error)
```

### `internal/project/loader.go`

Добавить `Project.ScheduledJobs []*metadata.ScheduledJob` + `loadScheduled()`.

### `internal/ui/admin.go`

```go
// GET /ui/admin/scheduled
func (s *Server) scheduledList(w, r) {
    jobs := s.scheduler.Jobs()
    s.render("scheduled_list", jobs)
}

// GET /ui/admin/scheduled/<name>
// Страница задания: расписание, последние 50 запусков, кнопки

// POST /ui/admin/scheduled/<name>/run-now
// Ручной запуск

// POST /ui/admin/scheduled/<name>/toggle
// Включить/выключить (если разрешено менять без перезапуска)
```

### Зависимость

Использовать `github.com/robfig/cron/v3` — стандартный cron-парсер для Go, без своего велосипеда.

---

## Тесты

### `internal/scheduler/scheduler_test.go`

```go
func TestScheduler_RunOnSchedule(t *testing.T) {
    // Использовать ускоренный clock для теста
    sched := scheduler.NewWithClock(testClock)
    sched.AddJob(&ScheduledJob{
        Name:     "test",
        Schedule: "* * * * *",   // каждую минуту
        Processor: "ТестоваяОбработка",
    })
    
    testClock.Advance(2 * time.Minute)
    runs, _ := sched.Runs(ctx, "test", 10)
    assert.Equal(t, 2, len(runs))
}

func TestScheduler_LogsToScheduledRuns(t *testing.T) { /* ... */ }

func TestScheduler_OnError_Continue(t *testing.T) {
    // Обработка падает → status=error, но следующий запуск состоится
}

func TestScheduler_Timeout_Kills(t *testing.T) {
    // Долгая обработка > timeout → status=timeout, контекст канселится
}
```

---

## Verification

1. В `examples/trade/scheduled/тестовоезадание.yaml` — задание запускающее обработку каждую минуту.
2. Через 2 минуты в `_scheduled_runs` — 2 записи с `status='success'`.
3. В UI «Регламентные задания» — видно расписание, последний запуск, длительность, output.
4. Клик «Запустить сейчас» → новая запись в журнале.
5. Выключение через UI → следующий запуск не происходит.
6. Сценарий ошибки: обработка вызывает `Error("...")` → запись с `status='error'`, текст ошибки.
7. `DEVELOPER.md` — раздел «Регламентные задания» с примерами расписаний.

---

## Эстимейт: 4 дня

- Scheduler + cron-парсер: 1 день
- Запуск обработок + лог в _scheduled_runs: 1 день
- UI (список, страница задания, история): 1.5 дня
- Тесты + пример: 0.5 дня
