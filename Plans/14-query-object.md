# Этап 14 — Объект Запрос в DSL

## Контекст

Отчёты используют язык запросов 1С→SQL (`internal/query/query.go`), но из DSL `*.os` файлов — нельзя. Это ломает естественный сценарий «при проведении расходной — проверить остаток через запрос». Сейчас разработчик вынужден либо писать ручные `SUM(CASE …)` агрегации, либо обходиться `РегистрыСведений.X.Получить` для одиночных значений.

Эта фича добавляет в DSL объект `Запрос`, который умеет выполнять язык запросов и возвращать результат как массив структур.

**Зависимости:** этап 12 (Массивы/Структуры) — результат запроса = массив структур.

---

## Синтаксис

```
Процедура ОбработкаПроведения()
    Запрос = Новый Запрос;
    Запрос.Текст =
        "ВЫБРАТЬ КоличествоОстаток
         ИЗ РегистрНакопления.ТоварноеДвижение.Остатки(&НаДату, Номенклатура = &Ном)";
    Запрос.УстановитьПараметр("НаДату", this.Дата);
    
    Для Каждого Строка Из this.Товары Цикл
        Запрос.УстановитьПараметр("Ном", Строка.Номенклатура);
        Результат = Запрос.Выполнить();
        
        Если Результат.Количество() = 0 ИЛИ Результат[0].КоличествоОстаток < Строка.Количество Тогда
            Error("Недостаточно товара: " + Строка.Номенклатура);
        КонецЕсли;
    КонецЦикла;
КонецПроцедуры
```

### Методы объекта Запрос

| Метод | Что делает |
|---|---|
| `Текст` (свойство) | Текст запроса на языке 1С |
| `УстановитьПараметр(имя, значение)` | Биндинг параметра `&Имя` |
| `Выполнить()` | Возвращает массив Структур |

Билингвальные алиасы: `Query.Text`, `SetParameter`, `Execute`.

---

## Изменения в коде

### `internal/dsl/interpreter/builtins.go`

При `Новый Запрос` — создать прокси-объект:

```go
type queryProxy struct {
    text   string
    params map[string]any
    svc    *runtime.Service  // содержит DB, Registry
    ctx    context.Context
}

func newQueryProxy(svc *runtime.Service, ctx context.Context) *queryProxy {
    return &queryProxy{
        params: make(map[string]any),
        svc:    svc,
        ctx:    ctx,
    }
}

func (q *queryProxy) SetMember(name string, value any) {
    if name == "Текст" || name == "Text" {
        q.text = toString(value)
    }
}

func (q *queryProxy) CallMethod(name string, args []any) any {
    switch name {
    case "УстановитьПараметр", "SetParameter":
        q.params[toString(args[0])] = args[1]
        return nil
    case "Выполнить", "Execute":
        return q.execute()
    }
    panic(...)
}

func (q *queryProxy) execute() *Array {
    res, err := query.Compile(q.text, query.CompileOpts{
        Params:    q.params,
        Registers: q.svc.Registry.Registers(),
        InfoRegs:  q.svc.Registry.InfoRegisters(),
    })
    if err != nil {
        panic(userError{Msg: "Ошибка запроса: " + err.Error()})
    }
    rows, err := q.svc.DB.QueryAll(q.ctx, res.SQL, res.Args...)
    if err != nil {
        panic(userError{Msg: "Ошибка выполнения SQL: " + err.Error()})
    }
    
    // Конвертируем []map[string]any → *Array of *Struct
    arr := &Array{}
    for _, row := range rows {
        s := &Struct{}
        for k, v := range row {
            s.Insert(k, v)
        }
        arr.items = append(arr.items, s)
    }
    return arr
}
```

### `internal/dsl/interpreter/env.go`

При создании окружения для исполнения DSL — добавлять `runtime.Service` и регистрировать обработчик `Новый Запрос`:

```go
func (env *Env) RegisterNewType(typeName string, factory func() any)

env.RegisterNewType("Запрос",  func() any { return newQueryProxy(svc, ctx) })
env.RegisterNewType("Query",   func() any { return newQueryProxy(svc, ctx) })
```

### `internal/storage/`

Метод `QueryAll` — общий способ выполнить произвольный SQL и получить `[]map[string]any`. Уже частично есть в `internal/storage/crud.go` (List), нужно вынести более общую функцию.

```go
func (db *DB) QueryAll(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
```

---

## Тесты

### `internal/dsl/interpreter/query_test.go` (integration с TEST_DATABASE_URL)

```go
func TestQuery_BasicExecute(t *testing.T) {
    // Подготовить данные в спр_номенклатура (3 записи)
    setupCatalog(t, "Номенклатура", 3)
    
    src := `Процедура Тест()
        Запрос = Новый Запрос;
        Запрос.Текст = "ВЫБРАТЬ Наименование ИЗ Справочник.Номенклатура";
        Возврат Запрос.Выполнить().Количество();
    КонецПроцедуры`
    assert.Equal(t, 3, evalProcedureWithService(src, "Тест", svc))
}

func TestQuery_WithParameter(t *testing.T) {
    src := `Процедура Тест()
        Запрос = Новый Запрос;
        Запрос.Текст = "ВЫБРАТЬ id ИЗ Справочник.Номенклатура ГДЕ Наименование = &Имя";
        Запрос.УстановитьПараметр("Имя", "Кабель");
        Возврат Запрос.Выполнить().Количество();
    КонецПроцедуры`
    assert.Equal(t, 1, evalProcedureWithService(src, "Тест", svc))
}

func TestQuery_VirtualTable_Balances(t *testing.T) {
    // Полный сценарий: проведение → запрос остатков → проверка
}
```

---

## Verification

1. В `examples/simple-erp/src/списание.posting.os` — добавить проверку остатка через `Запрос`. Попытка списать больше чем есть → понятная ошибка «Недостаточно товара».
2. Прогнать сценарий: создать поступление 10 шт → попытаться списать 15 → ошибка проведения с указанием на конкретную строку табличной части.
3. `DEVELOPER.md` — раздел «Объект Запрос в DSL» с примерами.

---

## Эстимейт: 3 дня (после этапа 12)

- Прокси-объект Запрос + методы: 1.5 дня
- Интеграция результата в Массив структур: 0.5 дня
- Тесты + пример: 1 день
