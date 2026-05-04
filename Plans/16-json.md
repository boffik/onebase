# Этап 16 — JSON в DSL

## Контекст

REST-интеграции, чтение настроек, обмен данными между базами. Сейчас в DSL никак нельзя ни прочитать, ни сериализовать JSON — а значит нельзя написать обработку, которая получает данные через HTTP или импортирует JSON-выгрузку.

Эта фича добавляет 2 встроенные функции: `ПрочитатьJSON` и `ЗаписатьJSON`.

**Зависимости:** этап 12 (Массивы/Структуры/Соответствия) — JSON object → Соответствие, JSON array → Массив.

---

## Синтаксис

### Парсинг

```
данные = ПрочитатьJSON("{""имя"":""Иван"",""возраст"":30}");
Сообщить(данные.Получить("имя"));        // Иван
Сообщить(данные.Получить("возраст"));    // 30

// Массив
arr = ПрочитатьJSON("[1,2,3]");
Сообщить(arr.Количество());              // 3
Сообщить(arr[0]);                         // 1

// Вложенные структуры
сложно = ПрочитатьJSON("{""товары"":[{""имя"":""A""},{""имя"":""B""}]}");
Для Каждого Товар Из сложно.Получить("товары") Цикл
    Сообщить(Товар.Получить("имя"));
КонецЦикла;
```

### Сериализация

```
м = Новый Соответствие;
м.Вставить("имя", "Иван");
м.Вставить("товары", Новый Массив);
текст = ЗаписатьJSON(м);                 // → {"имя":"Иван","товары":[]}
```

Билингвальный alias: `ReadJSON` / `WriteJSON`.

### Соответствие типов

| JSON | DSL |
|---|---|
| object | Соответствие |
| array | Массив |
| string | строка |
| number | число |
| true / false | bool |
| null | Неопределено |

---

## Изменения в коде

### `internal/dsl/interpreter/json_builtins.go` (новый)

```go
func builtinReadJSON(args []any) any {
    if len(args) != 1 {
        panic(userError{Msg: "ПрочитатьJSON: ожидается 1 аргумент"})
    }
    text, ok := args[0].(string)
    if !ok {
        panic(userError{Msg: "ПрочитатьJSON: аргумент должен быть строкой"})
    }
    
    var v any
    if err := json.Unmarshal([]byte(text), &v); err != nil {
        panic(userError{Msg: "ПрочитатьJSON: " + err.Error()})
    }
    return convertFromJSON(v)
}

func convertFromJSON(v any) any {
    switch x := v.(type) {
    case map[string]any:
        s := &Map{}  // или Struct, см. ниже
        for k, val := range x {
            s.Insert(k, convertFromJSON(val))
        }
        return s
    case []any:
        a := &Array{}
        for _, item := range x {
            a.Add(convertFromJSON(item))
        }
        return a
    case float64:
        // JSON числа приходят как float64
        if x == math.Floor(x) {
            return int64(x)
        }
        return x
    default:
        return v
    }
}

func builtinWriteJSON(args []any) any {
    raw, err := json.Marshal(toJSONValue(args[0]))
    if err != nil {
        panic(userError{Msg: "ЗаписатьJSON: " + err.Error()})
    }
    return string(raw)
}

func toJSONValue(v any) any {
    switch x := v.(type) {
    case *Map:    return x.AsMap()       // → map[string]any
    case *Struct: return x.AsMap()
    case *Array:  return x.AsSlice()     // → []any
    default:      return v
    }
}
```

### Регистрация в `env.go`

```go
env.RegisterFunc("ПрочитатьJSON", builtinReadJSON)
env.RegisterFunc("ReadJSON",      builtinReadJSON)
env.RegisterFunc("ЗаписатьJSON",  builtinWriteJSON)
env.RegisterFunc("WriteJSON",     builtinWriteJSON)
```

### Решение: Соответствие или Структура?

JSON object естественно ложится на **Соответствие** (произвольные ключи). Но для большинства случаев — фиксированный набор полей, тогда удобней `Структура` с доступом через точку (`данные.имя`).

Решение: возвращать `Соответствие`, но дать удобный helper `СоответствиеВСтруктуру(м)` или сразу делать `Структура` с `IsHashed=true` (поддержка обоих способов доступа).

---

## Тесты

### `internal/dsl/interpreter/json_test.go`

```go
func TestReadJSON_Object(t *testing.T) {
    src := `Процедура Тест()
        м = ПрочитатьJSON("{""a"":1,""b"":""two""}");
        Возврат м.Получить("a") = 1 И м.Получить("b") = "two";
    КонецПроцедуры`
    assert.True(t, evalProcedure(src, "Тест").(bool))
}

func TestReadJSON_Array(t *testing.T) {
    src := `Процедура Тест()
        arr = ПрочитатьJSON("[10,20,30]");
        Возврат arr.Количество() = 3 И arr[0] + arr[2] = 40;
    КонецПроцедуры`
    assert.True(t, evalProcedure(src, "Тест").(bool))
}

func TestReadJSON_Nested(t *testing.T) {
    src := `Процедура Тест()
        с = ПрочитатьJSON("{""товары"":[{""имя"":""A""}]}");
        товар = с.Получить("товары")[0];
        Возврат товар.Получить("имя");
    КонецПроцедуры`
    assert.Equal(t, "A", evalProcedure(src, "Тест"))
}

func TestWriteJSON_Roundtrip(t *testing.T) {
    src := `Процедура Тест()
        исх = "{""a"":1,""b"":[2,3]}";
        м = ПрочитатьJSON(исх);
        Возврат ЗаписатьJSON(м);
    КонецПроцедуры`
    out := evalProcedure(src, "Тест").(string)
    
    // Сравнение через парсинг (порядок ключей не гарантирован)
    var v1, v2 any
    json.Unmarshal([]byte(`{"a":1,"b":[2,3]}`), &v1)
    json.Unmarshal([]byte(out), &v2)
    assert.Equal(t, v1, v2)
}

func TestReadJSON_InvalidJSON_Error(t *testing.T) {
    // Неверный JSON → понятная ошибка, перехватываемая Попыткой
}
```

---

## Verification

1. В `examples/trade/processors/импортcjson.proc.os` — обработка читает JSON-строку и создаёт справочные элементы.
2. Простой сценарий: вход `[{"имя":"Поставщик1"},{"имя":"Поставщик2"}]` → создаются 2 контрагента.
3. Сценарий с ошибкой: невалидный JSON → ошибка перехватывается `Попыткой`, пользователю показывается понятное сообщение.
4. `DEVELOPER.md` — раздел «Работа с JSON».

---

## Эстимейт: 1 день (после этапа 12)

- Реализация `ПрочитатьJSON` / `ЗаписатьJSON`: 0.5 дня
- Тесты + пример: 0.5 дня
