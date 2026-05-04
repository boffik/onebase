# Этап 13 — Попытка / Исключение в DSL

## Контекст

Сейчас `Error("…")` отменяет всю транзакцию — нет способа поймать ошибку и обработать её. Это блокирует «безопасные» сценарии:

- «попробовать прочитать курс, если нет — взять 1.0»
- «массовое перепроведение: ошибка одного документа не должна прерывать обработку остальных»
- «импорт CSV: пропускать невалидные строки и продолжать»

Без try/except нет возможности писать устойчивый прикладной код. Также фича блокирует этап 15 (транзакции) — `НачатьТранзакцию` без `Попытка/Исключение` бесполезен.

---

## Синтаксис

```
Попытка
    курс = РегистрыСведений.КурсыВалют.ПолучитьПоследнее(this.Дата, "USD");
Исключение
    Сообщить("Курс не найден: " + ОписаниеОшибки());
    курс = Новый Структура("Курс", 1.0);
КонецПопытки;
```

Билингвальный alias: `Try / Except / EndTry`.

### `ОписаниеОшибки()`

Внутри `Исключение`-блока доступна глобальная функция `ОписаниеОшибки()` (alias `ErrorDescription()`) — возвращает текст исключения.

### Вложенные блоки

```
Попытка
    Попытка
        ОпасноеДействие();
    Исключение
        Сообщить("Внутренняя: " + ОписаниеОшибки());
        Error("Перевыброс");  // снова бросаем
    КонецПопытки;
Исключение
    Сообщить("Внешняя: " + ОписаниеОшибки());
КонецПопытки;
```

---

## Изменения в коде

### `internal/dsl/lexer/lexer.go`

Ключевые слова: `Попытка`, `Исключение`, `КонецПопытки`, `Try`, `Except`, `EndTry`.

### `internal/dsl/ast/ast.go`

```go
type TryStmt struct {
    Tok    token.Token
    Try    []Stmt
    Except []Stmt
}

func (*TryStmt) stmtNode() {}
func (*TryStmt) nodeType() string { return "TryStmt" }
```

### `internal/dsl/parser/parser.go`

Парсинг блока `Попытка <stmts> Исключение <stmts> КонецПопытки`.

### `internal/dsl/interpreter/interpreter.go`

```go
// Тип паники для пользовательских ошибок (Error / Ошибка).
type userError struct {
    Msg string
}

// Внутренние баги интерпретатора используют другой тип паники
// и НЕ перехватываются Попыткой — иначе сложно отлаживать.
type interpreterError struct{ ... }

func (i *Interpreter) execTry(t *ast.TryStmt) {
    func() {
        defer func() {
            if r := recover(); r != nil {
                if uerr, ok := r.(userError); ok {
                    // Положить описание в локальную переменную
                    i.env.Set("__error_description", uerr.Msg)
                    i.execBlock(t.Except)
                    return
                }
                // Не пользовательская ошибка — пробрасываем дальше
                panic(r)
            }
        }()
        i.execBlock(t.Try)
    }()
}
```

В `builtins.go` функция `Error` / `Ошибка`:

```go
func builtinError(args []any) any {
    panic(userError{Msg: toString(args[0])})
}
```

Функция `ОписаниеОшибки`:

```go
func builtinErrorDescription(env *Env) any {
    v, _ := env.Get("__error_description")
    return v  // string или nil
}
```

---

## Тесты

### `internal/dsl/interpreter/try_test.go`

```go
func TestTry_CatchesUserError(t *testing.T) {
    src := `Процедура Тест()
        x = 0;
        Попытка
            Error("упс");
        Исключение
            x = 1;
        КонецПопытки;
        Возврат x;
    КонецПроцедуры`
    assert.Equal(t, 1, evalProcedure(src, "Тест"))
}

func TestTry_NoError_SkipsExcept(t *testing.T) {
    src := `Процедура Тест()
        x = 0;
        Попытка
            x = 1;
        Исключение
            x = 99;
        КонецПопытки;
        Возврат x;
    КонецПроцедуры`
    assert.Equal(t, 1, evalProcedure(src, "Тест"))
}

func TestTry_Nested_InnerCatch(t *testing.T) {
    src := `Процедура Тест()
        Попытка
            Попытка
                Error("inner");
            Исключение
                Error("outer-from-inner");
            КонецПопытки;
        Исключение
            Возврат ОписаниеОшибки();
        КонецПопытки;
    КонецПроцедуры`
    assert.Equal(t, "outer-from-inner", evalProcedure(src, "Тест"))
}

func TestTry_ErrorDescription(t *testing.T) {
    src := `Процедура Тест()
        Попытка
            Error("моя ошибка");
        Исключение
            Возврат ОписаниеОшибки();
        КонецПопытки;
    КонецПроцедуры`
    assert.Equal(t, "моя ошибка", evalProcedure(src, "Тест"))
}
```

---

## Verification

1. В `examples/trade/processors/` создать обработку `массовоеперепроведение.proc.os` — каждый документ оборачивается в `Попытка/Исключение`, ошибка одного не прерывает обработку остальных, в результате — отчёт «X документов проведено, Y ошибок».
2. `DEVELOPER.md` — раздел «Обработка исключений» с примерами.

---

## Эстимейт: 2 дня

- Лексер + парсер: 0.5 дня
- Интерпретатор + `ОписаниеОшибки()`: 0.5 дня
- Тесты + пример: 1 день
