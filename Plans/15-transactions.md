# Этап 15 — Транзакции в DSL

## Контекст

Для обработок (массовое перепроведение, импорт CSV, миграция данных) нужна явная единица атомарности. Сейчас единственный неявный коммит — на `Save` одного документа. При сбое в середине обработки — половина данных уже записана, откатить нельзя.

Эта фича добавляет в DSL глобальные функции `НачатьТранзакцию`, `ЗафиксироватьТранзакцию`, `ОтменитьТранзакцию` с прокидыванием транзакции через context в storage.

**Зависимости:** этап 13 (Попытка/Исключение) — без try/except сценарий «откатить и продолжить» невозможен.

---

## Синтаксис

```
Процедура Выполнить()
    НачатьТранзакцию();
    Попытка
        Для Каждого Файл Из СписокФайлов Цикл
            ИмпортироватьИзJSON(Файл);
        КонецЦикла;
        ЗафиксироватьТранзакцию();
    Исключение
        ОтменитьТранзакцию();
        Сообщить("Откат: " + ОписаниеОшибки());
    КонецПопытки;
КонецПроцедуры
```

### Вложенные транзакции

Через `SAVEPOINT`:

```
НачатьТранзакцию();
// внешняя tx
    НачатьТранзакцию();  // SAVEPOINT sp1
    // внутренняя
    ОтменитьТранзакцию();  // ROLLBACK TO sp1
ЗафиксироватьТранзакцию();  // COMMIT внешней
```

Билингвальные алиасы: `BeginTransaction`, `CommitTransaction`, `RollbackTransaction`.

---

## Изменения в коде

### `internal/dsl/interpreter/transactions.go` (новый)

```go
type txStack struct {
    txs []pgx.Tx
}

func (s *txStack) Begin(ctx context.Context, db *storage.DB) error {
    if len(s.txs) == 0 {
        tx, err := db.Pool().Begin(ctx)
        if err != nil { return err }
        s.txs = append(s.txs, tx)
    } else {
        // Вложенная — SAVEPOINT
        spName := fmt.Sprintf("sp%d", len(s.txs))
        _, err := s.txs[len(s.txs)-1].Exec(ctx, "SAVEPOINT " + spName)
        if err != nil { return err }
        // Реиспользуем верхний tx, помечаем как nested через отдельный wrapper
        s.txs = append(s.txs, savepointTx{base: s.txs[len(s.txs)-1], name: spName})
    }
    return nil
}

func (s *txStack) Commit(ctx context.Context) error
func (s *txStack) Rollback(ctx context.Context) error
```

### `internal/dsl/interpreter/builtins.go`

```go
func builtinBeginTransaction(env *Env, args []any) any {
    svc := env.GetService()
    if err := svc.TxStack.Begin(env.Ctx(), svc.DB); err != nil {
        panic(userError{Msg: err.Error()})
    }
    // Установить tx в context для storage
    env.SetCtx(storage.WithTx(env.Ctx(), svc.TxStack.Top()))
    return nil
}

// аналогично commit / rollback
```

### `internal/storage/tx_context.go` (новый)

```go
type txCtxKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
    return context.WithValue(ctx, txCtxKey{}, tx)
}

func TxFromCtx(ctx context.Context) pgx.Tx {
    v, _ := ctx.Value(txCtxKey{}).(pgx.Tx)
    return v
}

// Универсальный execer: tx если есть, иначе pool
func (db *DB) execer(ctx context.Context) Execer {
    if tx := TxFromCtx(ctx); tx != nil { return tx }
    return db.pool
}
```

### `internal/storage/crud.go`, `register.go`, `inforeg.go`, `audit.go`, `numerator.go`, …

Все методы записи переходят на `db.execer(ctx)` вместо прямого `db.pool`. Это автоматически прокидывает транзакцию.

```go
// Было:
_, err := db.pool.Exec(ctx, sql, args...)

// Стало:
_, err := db.execer(ctx).Exec(ctx, sql, args...)
```

---

## Тесты

### `internal/dsl/interpreter/transaction_test.go` (integration)

```go
func TestTx_Commit(t *testing.T) {
    src := `Процедура Тест()
        НачатьТранзакцию();
        Создать("Контрагент", "Имя1");
        Создать("Контрагент", "Имя2");
        ЗафиксироватьТранзакцию();
    КонецПроцедуры`
    runProcedure(t, src, "Тест")
    
    count, _ := db.Count(ctx, "Контрагент")
    assert.Equal(t, 2, count)
}

func TestTx_Rollback_OnException(t *testing.T) {
    src := `Процедура Тест()
        НачатьТранзакцию();
        Попытка
            Создать("Контрагент", "Имя1");
            Error("rollback me");
            Создать("Контрагент", "Имя2");
        Исключение
            ОтменитьТранзакцию();
        КонецПопытки;
    КонецПроцедуры`
    runProcedure(t, src, "Тест")
    
    count, _ := db.Count(ctx, "Контрагент")
    assert.Equal(t, 0, count)  // оба отката
}

func TestTx_Nested_Savepoint(t *testing.T) {
    // Внешняя tx коммитится, внутренняя откатывается
}

func TestTx_NoExplicit_AutoCommit(t *testing.T) {
    // Запись без транзакции — автокоммит как раньше
}
```

---

## Verification

1. В `examples/trade/processors/` создать обработку `импортконтрагентовизcsv.proc.os`:
   - Импорт списка обернут в транзакцию
   - При ошибке хотя бы одной строки — весь импорт откатывается
2. Прогнать сценарий: 100 валидных строк + 1 невалидная → в БД 0 контрагентов, сообщение об ошибке.
3. Прогнать сценарий: все 101 валидные → в БД 101 контрагент.
4. `DEVELOPER.md` — раздел «Транзакции».

---

## Эстимейт: 2 дня (после этапа 13)

- Глобальные функции + стек tx: 0.5 дня
- Прокидывание tx через context в storage: 1 день
- Тесты + пример: 0.5 дня
