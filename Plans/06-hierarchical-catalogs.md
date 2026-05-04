# Этап 6 — Иерархические справочники

## Контекст

Все справочники сейчас плоские (`internal/storage/crud.go` без `parent_id`). При справочнике «Номенклатура» с тысячами позиций список становится неюзабельным. В 1С это решается флагом «Иерархический» с делением на «Группы» и «Элементы» — пользователь видит дерево, может сворачивать ветки, искать по конкретной группе.

Эта фича добавляет в платформу дерево с защитой от циклов и UI для раскрытия.

---

## YAML

```yaml
name: Номенклатура
hierarchical: true                  # включить иерархию
hierarchy_kind: groups_and_items    # или: groups_only (только группы=папки)
fields:
  - name: Наименование
    type: string
  - name: Артикул
    type: string
```

При `hierarchical: true` система автоматически добавляет:
- `parent_id UUID NULL REFERENCES <таблица>(id)` — родительская группа
- `is_folder BOOLEAN NOT NULL DEFAULT FALSE` — флаг «папка»

---

## DSL-доступ

```
// Поиск по наименованию (опционально внутри родителя)
гр  = Справочники.Номенклатура.НайтиПоНаименованию("Электроника");
тов = Справочники.Номенклатура.НайтиПоНаименованию("Кабель", гр);

// Доступ к родителю и флагу группы
this.Родитель             // ссылка на группу-родителя (UUID или Неопределено)
this.ЭтоГруппа            // bool (это is_folder)
```

---

## Хранилище

```sql
ALTER TABLE спр_<имя>
  ADD COLUMN IF NOT EXISTS parent_id UUID NULL REFERENCES спр_<имя>(id),
  ADD COLUMN IF NOT EXISTS is_folder BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_<имя>_parent ON спр_<имя>(parent_id);
```

### Защита от циклов

Recursive CTE на стороне Go перед записью:

```sql
WITH RECURSIVE chain AS (
    SELECT id, parent_id FROM спр_<имя> WHERE id = $newParent
    UNION ALL
    SELECT s.id, s.parent_id FROM спр_<имя> s
    JOIN chain c ON c.parent_id = s.id
)
SELECT 1 FROM chain WHERE id = $thisRecord LIMIT 1;
```

Если что-то нашлось — это цикл, отклоняем запись.

---

## Изменения в коде

### `internal/metadata/types.go`

```go
type Entity struct {
    ...
    Hierarchical    bool
    HierarchyKind   string  // "groups_and_items" | "groups_only"
}
```

### `internal/metadata/yaml.go`

Разбор полей `hierarchical`, `hierarchy_kind` (с дефолтом `groups_and_items`).

### `internal/storage/ddl.go`

При `EnsureSchema` для иерархического справочника — добавить `parent_id`, `is_folder` и индекс.

### `internal/storage/hierarchy.go` (новый)

```go
// Возвращает true если parentID создаёт цикл при назначении родителем для recordID.
func (db *DB) WouldCycle(ctx context.Context, entityName string, recordID, parentID uuid.UUID) (bool, error)

// Дочерние элементы (для AJAX-раскрытия дерева).
func (db *DB) Children(ctx context.Context, entityName string, parentID *uuid.UUID) ([]map[string]any, error)

// Полный путь от корня к элементу: ["Электроника", "Кабели", "USB"].
func (db *DB) Path(ctx context.Context, entityName string, recordID uuid.UUID) ([]string, error)
```

### `internal/storage/crud.go`

- При `Upsert` иерархического справочника проверять что `parent_id` ≠ `id` и нет цикла.
- При `Delete` — удаление папки с непустым содержимым → ошибка с понятным текстом.

### `internal/ui/handlers.go`

- Список иерархического справочника отображать деревом: показывать только корневые элементы (`parent_id IS NULL`), раскрытие через AJAX-запрос `/ui/catalog/<имя>/children?parent=<id>`.
- В форме редактирования — поле «Родитель» (combobox с поиском по группам того же справочника).
- Кнопки «Создать группу» и «Создать элемент» (для `groups_and_items`).
- Хлебные крошки в заголовке (через `db.Path`).

### `internal/dsl/interpreter/builtins.go`

- Метод `НайтиПоНаименованию(имя [, родитель])` для иерархических справочников.
- Свойства `Родитель`, `ЭтоГруппа` в прокси-объекте записи.

### `internal/launcher/configurator.go`

Чекбокс «Иерархический» в свойствах справочника + radio «Группы и элементы / Только группы».

---

## Тесты

### `internal/storage/hierarchy_test.go` (integration)

```go
func TestHierarchy_PreventCycle(t *testing.T) {
    a, _ := db.Create(ctx, "Номенклатура", map[string]any{
        "Наименование": "A", "is_folder": true,
    })
    b, _ := db.Create(ctx, "Номенклатура", map[string]any{
        "Наименование": "B", "parent_id": a, "is_folder": true,
    })
    err := db.Update(ctx, "Номенклатура", a, map[string]any{"parent_id": b})
    assert.ErrorContains(t, err, "цикл")  // A→B→A запрещено
}

func TestHierarchy_FindByName_InParent(t *testing.T) {
    grp, _ := db.Create(ctx, "Номенклатура", map[string]any{
        "Наименование": "Электроника", "is_folder": true,
    })
    db.Create(ctx, "Номенклатура", map[string]any{
        "Наименование": "Кабель USB", "parent_id": grp,
    })
    // Поиск без родителя — не находит
    found, _ := db.FindByName(ctx, "Номенклатура", "Кабель USB", nil)
    assert.Nil(t, found)
    // Поиск внутри Электроники — находит
    found, _ = db.FindByName(ctx, "Номенклатура", "Кабель USB", &grp)
    assert.NotNil(t, found)
}

func TestHierarchy_DeleteNonEmptyFolder_Fails(t *testing.T) { /* ... */ }
```

---

## Verification

1. В `examples/simple-erp/catalogs/номенклатура.yaml` добавить `hierarchical: true`.
2. Перезапуск `onebase dev` — таблица обновлена, в UI появляется кнопка-чевронка для раскрытия.
3. Создание группы «Электроника» → создание элементов внутри → элементы видны при раскрытии группы.
4. В `*.posting.os` — пример использования `Справочники.Номенклатура.НайтиПоНаименованию("Кабель", группаЭлектроника)`.
5. Попытка задать родителем самого себя — ошибка с понятным текстом.
6. `DEVELOPER.md` — раздел «Иерархические справочники».

---

## Эстимейт: 5–6 дней

- Метаданные + DDL: 0.5 дня
- Storage (валидация цикла, recursive CTE): 1 день
- UI дерева (раскрытие, отображение): 2 дня
- DSL методы: 0.5 дня
- Конфигуратор: 0.3 дня
- Тесты + примеры: 1 день
