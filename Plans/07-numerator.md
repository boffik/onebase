# Этап 7 — Автонумерация документов

## Контекст

Указана в roadmap проекта (`атомарная запись → автонумерация → проведение → REST API`). Сейчас номер документа вбивает пользователь руками — это неудобно и приводит к коллизиям при одновременной работе нескольких пользователей. В 1С автонумерация — базовая функция: задаёшь префикс и длину, получаешь `ПР-00000001`, `ПР-00000002`, …

---

## YAML

```yaml
name: Реализация
posting: true
numerator:
  prefix: "РТ-"          # "РТ-00000001"
  length: 8               # длина числовой части (с ведущими нулями)
  period: year            # сброс счётчика: year | month | none
fields:
  - name: Номер
    type: string
  - name: Дата
    type: date
```

### Поведение

- При создании документа без указания `Номер` (или с пустым `Номер`) — генерируется автоматически.
- Сброс счётчика на начало нового периода:
  - `year` — на 1 января (period_key = "2026")
  - `month` — на 1-е число месяца (period_key = "2026-05")
  - `none` — единый сквозной счётчик (period_key = "")
- Период определяется по полю типа `date` (первое в документе). Если поля `date` нет — берётся `now()`.
- Запись в `_numerators` транзакционная — `INSERT ... ON CONFLICT DO UPDATE RETURNING` атомарна, коллизии исключены.

---

## Хранилище

```sql
CREATE TABLE _numerators (
    entity_name TEXT NOT NULL,
    period_key  TEXT NOT NULL,    -- "" | "2026" | "2026-05"
    last_number INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (entity_name, period_key)
);
```

### `NextNumber(ctx, "Реализация", "2026")`

```sql
INSERT INTO _numerators (entity_name, period_key, last_number)
VALUES ($1, $2, 1)
ON CONFLICT (entity_name, period_key) DO UPDATE
  SET last_number = _numerators.last_number + 1
RETURNING last_number;
```

Затем форматирование на стороне Go: `prefix + lpad(last_number, length, '0')`.

---

## Изменения в коде

### `internal/metadata/types.go`

```go
type Numerator struct {
    Prefix string
    Length int
    Period string  // "year" | "month" | "none"
}

type Entity struct {
    ...
    Numerator *Numerator  // nil если автонумерация выключена
}
```

### `internal/metadata/yaml.go`

Разбор блока `numerator` с дефолтом `period: year`, `length: 8`.

### `internal/storage/numerator.go` (новый)

```go
func (db *DB) EnsureNumeratorSchema(ctx context.Context) error

// periodKey формируется в runtime по полю Дата
func (db *DB) NextNumber(ctx context.Context, entityName, periodKey string) (int, error)

// Форматирование числа: ("РТ-", 8, 42) → "РТ-00000042"
func FormatNumber(prefix string, length, number int) string
```

### `internal/runtime/` (или там, где сейчас выполняется create document)

Перед сохранением документа:
1. Если есть `Numerator` и `Номер` пуст — рассчитать `periodKey` от поля `Дата`.
2. Вызвать `NextNumber`, сформировать строку через `FormatNumber`.
3. Записать в `Номер`.

```go
func computePeriodKey(num *Numerator, doc map[string]any) string {
    if num.Period == "none" {
        return ""
    }
    date, _ := doc["Дата"].(time.Time)
    if date.IsZero() {
        date = time.Now()
    }
    if num.Period == "month" {
        return date.Format("2006-01")
    }
    return date.Format("2006")
}
```

### `internal/launcher/configurator.go`

Блок «Нумератор» в свойствах документа:
- Текстовое поле `prefix`
- Числовое поле `length`
- Dropdown `period: year | month | none`

---

## Тесты

### `internal/storage/numerator_test.go` (integration)

```go
func TestNumerator_Concurrent(t *testing.T) {
    var wg sync.WaitGroup
    nums := make([]int, 100)
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            n, _ := db.NextNumber(ctx, "Реализация", "2026")
            nums[i] = n
        }(i)
    }
    wg.Wait()
    sort.Ints(nums)
    for i, n := range nums {
        require.Equal(t, i+1, n)  // все номера уникальны от 1 до 100
    }
}

func TestNumerator_PeriodReset(t *testing.T) {
    n1, _ := db.NextNumber(ctx, "Реализация", "2026")
    n2, _ := db.NextNumber(ctx, "Реализация", "2025")
    assert.Equal(t, 1, n1)
    assert.Equal(t, 1, n2)  // отдельный счётчик на 2025
}

func TestFormatNumber(t *testing.T) {
    assert.Equal(t, "РТ-00000042", FormatNumber("РТ-", 8, 42))
    assert.Equal(t, "00001",       FormatNumber("",    5,  1))
}
```

---

## Verification

1. Добавить блок `numerator` в `examples/simple-erp/documents/поступление.yaml`.
2. Запуск `onebase dev` → создать 3 документа подряд → номера `ПОС-00000001`, `ПОС-00000002`, `ПОС-00000003`.
3. Создать документ датой следующего года → `ПОС-00000001` снова.
4. В Конфигураторе настроить нумератор для другого документа — должно работать.
5. Concurrent integration test — пройдены 100 одновременных создателей без коллизий.
6. `DEVELOPER.md` — раздел «Автонумерация».

---

## Эстимейт: 3 дня

- Метаданные + storage: 1 день
- Интеграция в runtime + UI (автозаполнение): 0.5 дня
- Конфигуратор: 0.5 дня
- Тесты + пример: 1 день
