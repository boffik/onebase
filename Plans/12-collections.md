# Этап 12 — Массивы / Структуры / Соответствия в DSL

## Контекст

В DSL нет промежуточных коллекций. По `internal/dsl/ast/ast.go` видно: только `Program`, `IfStmt`, `ForEach`, `BinaryExpr` — никаких `ArrayLit` или индексных обращений.

Без коллекций нельзя написать «собрать суммы по группам товаров и сравнить с лимитом» или «накопить ошибки и показать пользователю». Эта фича — основа для нескольких других этапов:

- Этап 14 (объект Запрос) — результат запроса = массив структур
- Этап 16 (JSON) — JSON-объект конвертируется в Соответствие/Массив
- Любая нетривиальная обработка — вычисления с промежуточным состоянием

---

## Синтаксис

### Массив

```
а = Новый Массив;
а.Добавить(1);
а.Добавить("строка");
Сообщить(а.Количество());           // 2
Сообщить(а[0]);                       // 1

Для Каждого Элемент Из а Цикл
    Сообщить(Элемент);
КонецЦикла;

а.Удалить(0);                         // удалить первый элемент
а.Очистить();                         // очистить весь массив
а.Вставить(0, "первый");              // вставить в позицию 0
```

### Структура (поля известны заранее)

```
с = Новый Структура("Имя, Возраст", "Иван", 30);
Сообщить(с.Имя);                     // Иван — доступ через точку
с.Вставить("Город", "Москва");        // динамическое добавление
Сообщить(с.Свойство("Город"));       // Москва — доступ по имени
с.Удалить("Город");
Если с.Свойство("Город") = Неопределено Тогда ... КонецЕсли;
```

### Соответствие (key-value, ключ — любой тип)

```
м = Новый Соответствие;
м.Вставить("USD", 90);
м.Вставить("EUR", 100);
Сообщить(м.Получить("USD"));         // 90

Для Каждого КЗ Из м Цикл
    Сообщить(КЗ.Ключ + " = " + КЗ.Значение);
КонецЦикла;

м.Удалить("USD");
Сообщить(м.Количество());            // 1
```

### Билингвальные алиасы

| Русский | English |
|---|---|
| `Новый Массив` / `Добавить` / `Количество` / `Удалить` / `Очистить` / `Вставить` | `New Array` / `Add` / `Count` / `Remove` / `Clear` / `Insert` |
| `Новый Структура` / `Свойство` | `New Structure` / `Property` |
| `Новый Соответствие` / `Получить` / `Вставить` | `New Map` / `Get` / `Insert` |

---

## Изменения в коде

### `internal/dsl/lexer/lexer.go`

- Распознавание ключевого слова `Новый` / `New`.
- Распознавание `[`, `]` для индексных обращений.

### `internal/dsl/ast/ast.go`

```go
type NewExpr struct {
    Tok      token.Token
    TypeName token.Token   // "Массив" / "Структура" / "Соответствие"
    Args     []Expr
}

type IndexExpr struct {
    Object Expr
    Index  Expr
}

func (*NewExpr) exprNode()    {}
func (*IndexExpr) exprNode()  {}
```

### `internal/dsl/parser/parser.go`

- Парсинг `Новый <Имя>` или `Новый <Имя>(<args>)` → `NewExpr`.
- Парсинг постфикса `<expr>[<expr>]` → `IndexExpr`.
- Также поддержать индекс как target в присваивании: `а[0] = "новое"`.

### `internal/dsl/interpreter/collections.go` (новый)

```go
type Array struct {
    items []any
}

func (a *Array) CallMethod(name string, args []any) any {
    switch name {
    case "Добавить", "Add":     a.items = append(a.items, args[0]); return nil
    case "Количество", "Count": return len(a.items)
    case "Удалить", "Remove":   /* по индексу */
    case "Очистить", "Clear":   a.items = nil; return nil
    case "Вставить", "Insert":  /* в позицию */
    }
    panic(...)
}

func (a *Array) Index(i any) any { return a.items[toInt(i)] }
func (a *Array) SetIndex(i any, v any) { a.items[toInt(i)] = v }
func (a *Array) Iterate() []any { return a.items }

type Struct struct {
    keys   []string         // сохраняем порядок вставки
    values map[string]any
}

type Map struct {
    keys   []any
    values map[any]any  // или []KV если ключи — не hashable
}
```

В интерпретаторе для `MemberExpr` — если объект `*Struct`, искать в `values` по имени поля.

### `internal/dsl/interpreter/interpreter.go`

- В `evalExpr` обработка `*NewExpr` — diSpatch по `TypeName`.
- В `evalExpr` обработка `*IndexExpr` — вызвать `Index` метод объекта.
- В `ForEachStmt` — для `*Map` итерация выдаёт пары `KV{Ключ, Значение}` (как `KeyValue` в 1С), для `*Array` — элементы, для `*Struct` — пары `(имя, значение)`.

---

## Тесты

### `internal/dsl/interpreter/collections_test.go`

```go
func TestArray_AddCountIndex(t *testing.T) {
    src := `Процедура Тест()
        а = Новый Массив;
        а.Добавить("x");
        а.Добавить("y");
        Возврат а.Количество() = 2 И а[0] = "x";
    КонецПроцедуры`
    assert.True(t, evalProcedure(src, "Тест").(bool))
}

func TestArray_IndexAssign(t *testing.T) {
    src := `Процедура Тест()
        а = Новый Массив;
        а.Добавить(1);
        а[0] = 99;
        Возврат а[0];
    КонецПроцедуры`
    assert.Equal(t, 99, evalProcedure(src, "Тест"))
}

func TestStruct_AccessByDotAndProperty(t *testing.T) {
    src := `Процедура Тест()
        с = Новый Структура("Имя", "Иван");
        Возврат с.Имя = "Иван" И с.Свойство("Имя") = "Иван";
    КонецПроцедуры`
    assert.True(t, evalProcedure(src, "Тест").(bool))
}

func TestMap_GetSet(t *testing.T) { /* ... */ }

func TestForEach_Map_KeyValue(t *testing.T) {
    src := `Процедура Тест()
        м = Новый Соответствие;
        м.Вставить("a", 1);
        м.Вставить("b", 2);
        сумма = 0;
        Для Каждого КЗ Из м Цикл
            сумма = сумма + КЗ.Значение;
        КонецЦикла;
        Возврат сумма;
    КонецПроцедуры`
    assert.Equal(t, 3, evalProcedure(src, "Тест"))
}

func TestArray_OutOfBounds_Error(t *testing.T) {
    // обращение к а[10] при размере 2 → понятная ошибка
}
```

---

## Verification

1. В `examples/trade/processors/` создать обработку `проверкацен.proc.os` использующую Соответствие для накопления отклонений по группам товаров.
2. Сообщения об ошибках при индексе вне границ — понятные (например «Индекс 10 за пределами массива размера 2»).
3. `DEVELOPER.md` — раздел «Коллекции в DSL» с примерами Массив, Структура, Соответствие.

---

## Эстимейт: 4 дня

- Лексер/парсер `Новый`, `[]`: 1 день
- AST + интерпретатор Массив (Array): 1 день
- Структура + Соответствие: 1 день
- Интеграция в ForEach + тесты + пример: 1 день
