# Этап 8 — Журнал документов

## Контекст

В 1С — общий список разнотипных документов с фильтрами по дате/типу/контрагенту. Сейчас в onebase каждый документ открывается через свой пункт навигации, сводного списка нет — пользователь вынужден переключаться между «Поступлениями» и «Реализациями» вручную.

Эта фича добавляет новый объект конфигурации — журнал, объединяющий несколько документов в единую таблицу.

---

## YAML

`journals/всеоперации.yaml`:

```yaml
name: ВсеОперации
title: Все операции
documents:
  - Поступление
  - Реализация
  - Списание
columns:
  - field: Дата
    label: Дата
  - field: Номер
    label: Номер
  - field: Контрагент            # отображать Поставщик/Покупатель как Контрагент
    fallback: [Поставщик, Покупатель]
  - field: Сумма
    label: Сумма
    format: "number:2"
filters:
  - field: Дата
    type: date_range
  - field: Контрагент
    type: reference:Контрагент
```

### Семантика fallback

Колонка `Контрагент` соответствует разным полям в разных документах. `fallback: [Поставщик, Покупатель]` означает «возьми первое непустое из этих полей». В SQL — `COALESCE`.

---

## SQL под капотом

`UNION ALL` по таблицам выбранных документов с приведением графов к общему виду:

```sql
SELECT 'Поступление' AS doc_kind, id, Дата, Номер,
       Поставщик AS Контрагент, Сумма
FROM док_поступление
UNION ALL
SELECT 'Реализация',           id, Дата, Номер,
       Покупатель,             Сумма
FROM док_реализация
UNION ALL
SELECT 'Списание',             id, Дата, Номер,
       NULL,                   Сумма
FROM док_списание
ORDER BY Дата DESC
LIMIT 100 OFFSET ?
```

Если поле задано через `fallback` — `COALESCE(<f1>, <f2>, ...)` AS <колонка>.

Фильтры применяются как `WHERE` либо к каждому подзапросу (если фильтр по общему полю — Дата), либо после UNION в обёртке.

---

## Изменения в коде

### `journals/<имя>.yaml`

Новый каталог в проекте.

### `internal/metadata/journal.go` (новый)

```go
type Journal struct {
    Name      string
    Title     string
    Documents []string
    Columns   []JournalColumn
    Filters   []JournalFilter
}

type JournalColumn struct {
    Field    string
    Label    string
    Fallback []string  // имена полей в исходных документах
    Format   string    // "number:2", "date", ...
}

type JournalFilter struct {
    Field string
    Type  string  // "date_range" | "reference:X" | "string"
}

func LoadJournalFile(path string) (*Journal, error)
func LoadJournalDir(dir string) ([]*Journal, error)
```

### `internal/project/loader.go`

```go
type Project struct {
    ...
    Journals []*metadata.Journal
}

func (p *Project) loadJournals() error {
    dir := filepath.Join(p.Dir, "journals")
    js, err := metadata.LoadJournalDir(dir)
    if os.IsNotExist(err) { return nil }
    if err != nil { return err }
    p.Journals = js
    return nil
}
```

### `internal/storage/journal.go` (новый)

```go
func (db *DB) JournalQuery(
    ctx context.Context,
    j *metadata.Journal,
    docs map[string]*metadata.Entity,  // для разрешения полей
    filter map[string]any,
    params ListParams,
) ([]map[string]any, int, error)
```

Динамическая генерация UNION ALL c учётом fallback и фильтров. Возвращает строки + total count для пагинации.

### `internal/ui/handlers.go`

```go
// GET /ui/journal/<имя>
func (s *Server) journalList(w http.ResponseWriter, r *http.Request) {
    name := chi.URLParam(r, "name")
    j := s.reg.GetJournal(name)
    if j == nil { http.Error(w, "journal not found", 404); return }
    
    filter := parseFilters(r, j)
    rows, total, _ := s.store.JournalQuery(r.Context(), j, s.reg.Documents(), filter, params)
    s.renderJournal(w, j, rows, total)
}
```

Клик на запись → редирект на страницу исходного документа `/ui/document/<doc_kind>/<id>`.

### `internal/ui/server.go` (`buildNav`)

Добавить группу «Журналы» → пункты для каждого journal'а.

### `internal/runtime/registry.go`

```go
type Registry struct {
    ...
    journals map[string]*metadata.Journal
}

func (r *Registry) GetJournal(name string) *metadata.Journal
func (r *Registry) Journals() []*metadata.Journal
```

### `internal/launcher/configurator.go`

Раздел «Журналы» в дереве — отображение списка, при клике YAML-содержимое.

---

## Тесты

### `internal/storage/journal_test.go` (integration)

```go
func TestJournal_UnionAll(t *testing.T) {
    // создать 2 поступления и 1 реализацию
    j := &metadata.Journal{
        Name: "ВсеОперации",
        Documents: []string{"Поступление", "Реализация"},
        Columns: []metadata.JournalColumn{
            {Field: "Дата"}, {Field: "Номер"},
            {Field: "Контрагент", Fallback: []string{"Поставщик", "Покупатель"}},
        },
    }
    rows, total, err := db.JournalQuery(ctx, j, docs, nil, ListParams{})
    require.NoError(t, err)
    assert.Equal(t, 3, total)
    // проверить что Контрагент заполнен из правильного поля для каждой строки
}

func TestJournal_FilterByDate(t *testing.T) { /* ... */ }
func TestJournal_FilterByContragent(t *testing.T) { /* ... */ }
```

---

## Verification

1. Создать `examples/simple-erp/journals/всеоперации.yaml`.
2. В nav появилась группа «Журналы» → «Все операции».
3. На странице — все документы вперемешку, отсортированы по дате убыв.
4. Фильтр «Дата с/по» сужает выборку, фильтр «Контрагент» — тоже.
5. Клик на запись Поступление № 5 → открывается `/ui/document/Поступление/<id>`.
6. `DEVELOPER.md` — раздел «Журналы документов».

---

## Эстимейт: 3–4 дня

- Метаданные + загрузка: 0.5 дня
- Storage UNION ALL генератор: 1 день
- UI (таблица + фильтры + пагинация): 1.5 дня
- Тесты + пример: 0.5 дня
