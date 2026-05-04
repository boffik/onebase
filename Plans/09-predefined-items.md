# Этап 9 — Предопределённые элементы справочников

## Контекст

Сейчас фиксированные справочные значения создаются вручную после развёртывания: валюта «Рубль», статус заказа «Новый», виды контрагентов. Это хрупко — пропустил один элемент при инициализации, код в DSL ищет его и падает. В 1С предопределённые элементы — часть конфигурации: задаются в YAML, синхронизируются в БД при старте, доступны по имени из DSL без поиска.

---

## YAML

```yaml
name: Валюта
fields:
  - name: Код
    type: string
  - name: Наименование
    type: string
predefined:
  - name: Рубль
    fields: { Код: "RUB", Наименование: "Российский рубль" }
  - name: Доллар
    fields: { Код: "USD", Наименование: "Доллар США" }
  - name: Евро
    fields: { Код: "EUR", Наименование: "Евро" }
```

---

## DSL-доступ

```
// Прямой доступ
рубль = ПредопределённыеЗначения.Валюта.Рубль;

// Сравнение
Если this.Валюта = ПредопределённыеЗначения.Валюта.Рубль Тогда
    ...
КонецЕсли;

// В присваивании
this.ВалютаУчёта = ПредопределённыеЗначения.Валюта.Рубль;
```

Билингвальный alias: `PredefinedValues.Валюта.Рубль`.

---

## Хранилище

При миграции конфигурации — для справочников с `predefined`:

```sql
ALTER TABLE спр_<имя>
  ADD COLUMN IF NOT EXISTS _predefined_name TEXT,
  ADD COLUMN IF NOT EXISTS _is_predefined BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_<имя>_predefined
  ON спр_<имя>(_predefined_name) WHERE _is_predefined = TRUE;
```

Затем upsert каждого предопределённого:

```sql
INSERT INTO спр_валюта (id, _predefined_name, _is_predefined, Код, Наименование)
VALUES (gen_random_uuid(), 'Рубль', TRUE, 'RUB', 'Российский рубль')
ON CONFLICT (_predefined_name) WHERE _is_predefined = TRUE DO UPDATE
  SET Код = EXCLUDED.Код, Наименование = EXCLUDED.Наименование;
```

При повторном sync — id не меняется, обновляются только поля.

---

## Изменения в коде

### `internal/metadata/types.go`

```go
type Predefined struct {
    Name   string                  // имя для DSL: "Рубль"
    Fields map[string]any          // значения полей
}

type Entity struct {
    ...
    Predefined []*Predefined
}
```

### `internal/storage/predefined.go` (новый)

```go
func (db *DB) EnsurePredefinedColumns(ctx context.Context, entities []*metadata.Entity) error
func (db *DB) SyncPredefined(ctx context.Context, e *metadata.Entity) error
func (db *DB) GetPredefinedID(ctx context.Context, entityName, predefinedName string) (uuid.UUID, error)
```

При старте `dev/run` — `SyncPredefined` для всех справочников с `Predefined != nil`.

### `internal/dsl/interpreter/env.go`

- Глобальный объект `ПредопределённыеЗначения` (alias `PredefinedValues`).
- При обращении `ПредопределённыеЗначения.Валюта` возвращается прокси справочника.
- При `.Рубль` — прокси разрешает имя через `GetPredefinedID` и возвращает ссылку на запись.

```go
type predefinedProxy struct {
    entityName string
    db         *storage.DB
    ctx        context.Context
}

func (p *predefinedProxy) GetMember(name string) any {
    id, err := p.db.GetPredefinedID(p.ctx, p.entityName, name)
    if err != nil {
        panic(userError{Msg: "Предопределённый элемент " + p.entityName + "." + name + " не найден"})
    }
    return refValue{EntityName: p.entityName, ID: id}
}
```

### `internal/storage/crud.go`

При попытке `Delete` записи с `_is_predefined = TRUE` — ошибка «Нельзя удалить предопределённый элемент».

### `internal/ui/`

- На странице списка справочника — иконка ★ для предопределённых элементов.
- В форме редактирования — поля для редактирования (значения можно менять, имя — нельзя).
- Кнопка «Удалить» для предопределённого элемента — disabled с tooltip.

### `internal/launcher/configurator.go`

Раздел «Предопределённые» в свойствах справочника — список с возможностью добавлять/удалять/редактировать.

---

## Тесты

### `internal/storage/predefined_test.go` (integration)

```go
func TestPredefined_Sync(t *testing.T) {
    e := &metadata.Entity{
        Name: "Валюта",
        Fields: []metadata.Field{{Name: "Код", Type: "string"}},
        Predefined: []*Predefined{
            {Name: "Рубль", Fields: map[string]any{"Код": "RUB"}},
        },
    }
    db.EnsurePredefinedColumns(ctx, []*metadata.Entity{e})
    db.SyncPredefined(ctx, e)
    
    id1, _ := db.GetPredefinedID(ctx, "Валюта", "Рубль")
    require.NotEqual(t, uuid.Nil, id1)
    
    // Повторный sync — id не меняется, поля обновляются
    e.Predefined[0].Fields["Код"] = "RUB-NEW"
    db.SyncPredefined(ctx, e)
    id2, _ := db.GetPredefinedID(ctx, "Валюта", "Рубль")
    assert.Equal(t, id1, id2)
    
    rec, _ := db.GetByID(ctx, "Валюта", id2, e)
    assert.Equal(t, "RUB-NEW", rec["Код"])
}

func TestPredefined_CannotDelete(t *testing.T) {
    err := db.Delete(ctx, "Валюта", id)
    assert.ErrorContains(t, err, "предопределённый")
}
```

---

## Verification

1. В `examples/trade/catalogs/валюта.yaml` добавить `predefined` блок с тремя валютами.
2. Запуск `onebase dev` — в таблице `спр_валюта` автоматически появляются 3 записи с `_is_predefined = TRUE`.
3. В DSL `examples/trade/src/реализациятоваров.os`:
   ```
   Сообщить(ПредопределённыеЗначения.Валюта.Рубль.Наименование);
   ```
   → «Российский рубль».
4. В UI попытка удалить «Рубль» → ошибка.
5. Изменить YAML (поправить наименование валюты), перезапуск → значение обновлено в БД, id тот же.
6. `DEVELOPER.md` — раздел «Предопределённые элементы».

---

## Эстимейт: 2–3 дня

- Метаданные + sync: 0.5 дня
- DSL-объект `ПредопределённыеЗначения`: 0.5 дня
- UI (защита от удаления + иконка): 0.5 дня
- Тесты + пример: 0.5 дня
- Конфигуратор: 0.3 дня
