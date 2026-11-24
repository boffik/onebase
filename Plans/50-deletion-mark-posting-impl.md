# План 50 — реализация: пометка на удаление ↔ проведение (issue #36)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Сделать «проведён» и «помечен на удаление» взаимоисключающими состояниями документа (как в 1С): пометка проведённого документа авто-отменяет проведение, проведение помеченного запрещено, плюс списочные действия и DSL-методы отмены проведения / снятия пометки.

**Architecture:** Вариант A — точечные guard'ы + переиспользование существующих примитивов. Новый storage-инвариант `ErrPostingDeletionMarked` + хелпер `IsMarkedForDeletion`; backstop в `SetPosted`; ранние guard'ы в трёх точках входа проведения; общий `(*Server).markForDeletion` поверх существующих `clearMovements`/`SetPosted`/`MarkForDeletion`; DSL-методы и пункты контекстного меню как тонкие обёртки.

**Tech Stack:** Go (без CGo), PostgreSQL/SQLite за интерфейсом `storage.Dialect`, тесты на SQLite через `storage.ConnectSQLite`. DSL-движок (`internal/dsl`), managed/autogen HTML-шаблоны (`internal/ui`).

**Дизайн (спека):** [Plans/50-deletion-mark-posting.md](50-deletion-mark-posting.md)

---

## Файловая структура (что трогаем)

| Файл | Что меняем |
|---|---|
| `internal/storage/deletion.go` | + `ErrPostingDeletionMarked`, + `IsMarkedForDeletion` |
| `internal/storage/crud.go` | backstop в `SetPosted` (отклонять `posted=true` для помеченного) |
| `internal/storage/deletion_posting_test.go` | **новый** — тесты storage-инварианта |
| `internal/entityservice/service.go` | ранний guard в `Save` (ветка `isPosting`) |
| `internal/entityservice/posting_mark_test.go` | **новый** — тест guard'а формы-сабмита |
| `internal/ui/handlers.go` | guard в `postDocument`; `+ vals["deletion_mark"]`; `+ (*Server).markForDeletion`; правка `deleteRecord` (авто-отмена при пометке + ветка `mark=0`) |
| `internal/ui/dsl_documents.go` | guard в `docWriter.post`; + методы `ОтменитьПроведение`/`ПометитьНаУдаление`/`СнятьПометку` + хелперы `unpostRef`/`markRef`; + import `storage` |
| `internal/ui/templates.go` | скрыть «Провести» у помеченного; кнопка «Снять пометку»; списочное меню (отмена проведения / снятие пометки) + data-атрибуты + `_canUnpost` |
| `internal/ui/templates_managed.go` | то же для managed-формы |
| `internal/ui/deletion_posting_test.go` | **новый** — тесты `markForDeletion`, guard, DSL-методы (+ общий хелпер) |
| `Plans/50-deletion-mark-posting.md`, `Plans/README.md` | статус → реализовано |

---

## Task 1: storage — инвариант проведения (`IsMarkedForDeletion`, `ErrPostingDeletionMarked`, backstop в `SetPosted`)

**Files:**
- Modify: `internal/storage/deletion.go:1-43`
- Modify: `internal/storage/crud.go:630-636`
- Test: `internal/storage/deletion_posting_test.go` (create)

- [ ] **Step 1: Написать падающий тест**

Create `internal/storage/deletion_posting_test.go`:

```go
package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/boffik/onebase/internal/metadata"
)

func TestIsMarkedForDeletionAndPostingGuard(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	doc := &metadata.Entity{
		Name:    "Расходник",
		Kind:    metadata.KindDocument,
		Posting: true,
		Fields:  []metadata.Field{{Name: "Номер", Type: metadata.FieldTypeString}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{doc}); err != nil {
		t.Fatal(err)
	}

	id := uuid.New()
	if err := db.Upsert(ctx, doc.Name, id, map[string]any{"Номер": "Р-1"}, doc); err != nil {
		t.Fatal(err)
	}

	// Не помечен → false.
	if marked, err := db.IsMarkedForDeletion(ctx, doc.Name, id); err != nil || marked {
		t.Fatalf("ожидался (false,nil), получили (%v,%v)", marked, err)
	}
	// Несуществующий id → false без ошибки.
	if marked, err := db.IsMarkedForDeletion(ctx, doc.Name, uuid.New()); err != nil || marked {
		t.Fatalf("несуществующий: ожидался (false,nil), получили (%v,%v)", marked, err)
	}
	// До пометки SetPosted(true) работает.
	if err := db.SetPosted(ctx, doc.Name, id, true); err != nil {
		t.Fatalf("SetPosted(true) до пометки: %v", err)
	}
	// Снять проведение, пометить на удаление.
	if err := db.SetPosted(ctx, doc.Name, id, false); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkForDeletion(ctx, doc.Name, id, true); err != nil {
		t.Fatal(err)
	}
	if marked, _ := db.IsMarkedForDeletion(ctx, doc.Name, id); !marked {
		t.Fatal("после MarkForDeletion ожидался marked=true")
	}
	// SetPosted(true) на помеченном → ErrPostingDeletionMarked.
	if err := db.SetPosted(ctx, doc.Name, id, true); !errors.Is(err, ErrPostingDeletionMarked) {
		t.Fatalf("ожидался ErrPostingDeletionMarked, получили %v", err)
	}
	// SetPosted(false) (отмена проведения) на помеченном всё ещё работает.
	if err := db.SetPosted(ctx, doc.Name, id, false); err != nil {
		t.Fatalf("SetPosted(false) на помеченном должен работать: %v", err)
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что не компилируется/падает**

Run: `go test ./internal/storage/ -run TestIsMarkedForDeletionAndPostingGuard`
Expected: FAIL — `db.IsMarkedForDeletion` / `ErrPostingDeletionMarked` undefined.

- [ ] **Step 3: Добавить `ErrPostingDeletionMarked` и `IsMarkedForDeletion`**

In `internal/storage/deletion.go`, заменить блок импортов (строки 3-10):

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/boffik/onebase/internal/metadata"
)
```

И сразу после блока импортов (перед `// EnsureDeletionMark`) добавить:

```go
// ErrPostingDeletionMarked возвращается при попытке провести документ, помеченный
// на удаление. Проведённость и пометка на удаление взаимоисключающи (как в 1С).
var ErrPostingDeletionMarked = errors.New("документ помечен на удаление: проведение невозможно")

// IsMarkedForDeletion сообщает, выставлен ли deletion_mark у записи.
// Возвращает (false, nil), если записи нет.
func (db *DB) IsMarkedForDeletion(ctx context.Context, entityName string, id uuid.UUID) (bool, error) {
	d := db.dialect
	var mark bool
	err := db.QueryRow(ctx,
		fmt.Sprintf("SELECT deletion_mark FROM %s WHERE id = %s",
			metadata.TableName(entityName), d.Placeholder(1)),
		idArg(d, id),
	).Scan(&mark)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return mark, nil
}
```

- [ ] **Step 4: Добавить backstop в `SetPosted`**

In `internal/storage/crud.go`, заменить начало `SetPosted` (строки 631-636):

```go
func (db *DB) SetPosted(ctx context.Context, entityName string, id uuid.UUID, posted bool) error {
	d := db.dialect
	err := db.exec(ctx,
		fmt.Sprintf("UPDATE %s SET posted = %s WHERE id = %s",
			metadata.TableName(entityName), d.Placeholder(1), d.Placeholder(2)),
		posted, idArg(d, id))
```

на:

```go
func (db *DB) SetPosted(ctx context.Context, entityName string, id uuid.UUID, posted bool) error {
	d := db.dialect
	if posted {
		// Инвариант: помеченный на удаление документ нельзя провести. Backstop —
		// точки входа проведения сторожат раньше, это страховка от будущих путей.
		if marked, mErr := db.IsMarkedForDeletion(ctx, entityName, id); mErr != nil {
			return mErr
		} else if marked {
			return ErrPostingDeletionMarked
		}
	}
	err := db.exec(ctx,
		fmt.Sprintf("UPDATE %s SET posted = %s WHERE id = %s",
			metadata.TableName(entityName), d.Placeholder(1), d.Placeholder(2)),
		posted, idArg(d, id))
```

- [ ] **Step 5: Запустить тест — убедиться, что проходит**

Run: `go test ./internal/storage/ -run TestIsMarkedForDeletionAndPostingGuard -v`
Expected: PASS

- [ ] **Step 6: Коммит**

```bash
git add internal/storage/deletion.go internal/storage/crud.go internal/storage/deletion_posting_test.go
git commit -m "feat(storage): инвариант — нельзя провести помеченный на удаление (#36)"
```

---

## Task 2: entityservice — запрет проведения помеченного при сабмите формы

**Files:**
- Modify: `internal/entityservice/service.go:150-156`
- Test: `internal/entityservice/posting_mark_test.go` (create)

- [ ] **Step 1: Написать падающий тест**

Create `internal/entityservice/posting_mark_test.go`:

```go
package entityservice

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/boffik/onebase/internal/dsl/interpreter"
	"github.com/boffik/onebase/internal/metadata"
	"github.com/boffik/onebase/internal/runtime"
	"github.com/boffik/onebase/internal/storage"
)

func TestSave_PostingBlockedWhenMarked(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	doc := &metadata.Entity{
		Name:    "Расходник",
		Kind:    metadata.KindDocument,
		Posting: true,
		Fields:  []metadata.Field{{Name: "Номер", Type: metadata.FieldTypeString}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{doc}); err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if err := db.Upsert(ctx, doc.Name, id, map[string]any{"Номер": "Р-1"}, doc); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkForDeletion(ctx, doc.Name, id, true); err != nil {
		t.Fatal(err)
	}

	reg := runtime.NewRegistry()
	reg.Load(runtime.LoadOptions{Entities: []*metadata.Entity{doc}})
	svc := &Service{Store: db, Reg: reg, Interp: interpreter.New()}

	res, err := svc.Save(ctx, SaveRequest{
		Entity: doc, ID: id, IsNew: false,
		Fields: map[string]any{"Номер": "Р-1"}, Action: "post",
	})
	if err != nil {
		t.Fatalf("ожидалась бизнес-ошибка через DSLError, получили err=%v", err)
	}
	if res.DSLError == "" {
		t.Fatal("ожидался res.DSLError != \"\" (проведение помеченного запрещено)")
	}
	var posted bool
	db.QueryRow(ctx, "SELECT posted FROM расходник LIMIT 1").Scan(&posted)
	if posted {
		t.Error("документ не должен быть проведён")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `go test ./internal/entityservice/ -run TestSave_PostingBlockedWhenMarked`
Expected: FAIL — `res.DSLError == ""` (guard ещё нет; Save проведёт документ).

- [ ] **Step 3: Добавить guard в `Save`**

In `internal/entityservice/service.go`, заменить строки 150-156:

```go
	// Выбор хука: OnPost при проведении документа, иначе OnWrite.
	isPosting := req.Entity.Posting && (req.Action == "post" || req.Action == "post_and_close")
	hookName := "OnWrite"
	if isPosting {
		hookName = "OnPost"
	}
	proc := s.Reg.GetProcedure(req.Entity.Name, hookName)
```

на:

```go
	// Выбор хука: OnPost при проведении документа, иначе OnWrite.
	isPosting := req.Entity.Posting && (req.Action == "post" || req.Action == "post_and_close")
	// Инвариант: помеченный на удаление документ нельзя провести (как в 1С).
	// Проверяем ДО запуска хука и записи, чтобы не терять правки полей.
	if isPosting && !req.IsNew {
		marked, err := s.Store.IsMarkedForDeletion(ctx, req.Entity.Name, req.ID)
		if err != nil {
			return SaveResult{}, err
		}
		if marked {
			return SaveResult{ID: req.ID, DSLError: storage.ErrPostingDeletionMarked.Error()}, nil
		}
	}
	hookName := "OnWrite"
	if isPosting {
		hookName = "OnPost"
	}
	proc := s.Reg.GetProcedure(req.Entity.Name, hookName)
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `go test ./internal/entityservice/ -run TestSave_PostingBlockedWhenMarked -v`
Expected: PASS

- [ ] **Step 5: Коммит**

```bash
git add internal/entityservice/service.go internal/entityservice/posting_mark_test.go
git commit -m "feat(entityservice): запрет проведения помеченного документа при сабмите (#36)"
```

---

## Task 3: ui — guard'ы проведения (HTTP `postDocument`, DSL `docWriter.post`) + общий тест-харнесс

**Files:**
- Modify: `internal/ui/handlers.go:900-905`
- Modify: `internal/ui/dsl_documents.go:1-13` (import) и `:428-435` (post)
- Test: `internal/ui/deletion_posting_test.go` (create — харнесс + тест guard'а)

- [ ] **Step 1: Создать тест-файл с общим харнессом и падающим тестом guard'а**

Create `internal/ui/deletion_posting_test.go`:

```go
package ui

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/boffik/onebase/internal/dsl/ast"
	"github.com/boffik/onebase/internal/dsl/interpreter"
	"github.com/boffik/onebase/internal/metadata"
	"github.com/boffik/onebase/internal/runtime"
	"github.com/boffik/onebase/internal/storage"
)

// newPostingDoc поднимает SQLite-Server с проводимым документом ПоступлениеТоваров
// (ТЧ Товары), который в ОбработкаПроведения пишет приход в регистр ОстаткиТоваров.
func newPostingDoc(t *testing.T) (context.Context, *storage.DB, *Server, *docProxy, *metadata.Entity) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	doc := &metadata.Entity{
		Name:    "ПоступлениеТоваров",
		Kind:    metadata.KindDocument,
		Posting: true,
		Fields:  []metadata.Field{{Name: "Номер", Type: metadata.FieldTypeString}},
		TableParts: []metadata.TablePart{{Name: "Товары", Fields: []metadata.Field{
			{Name: "Номенклатура", Type: metadata.FieldTypeString},
			{Name: "Количество", Type: metadata.FieldTypeNumber},
		}}},
	}
	reg := &metadata.Register{
		Name:       "ОстаткиТоваров",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{doc}); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateRegisters(ctx, []*metadata.Register{reg}); err != nil {
		t.Fatal(err)
	}

	onPostSrc := `Процедура ОбработкаПроведения()
  Для Каждого Стр Из ЭтотОбъект.Товары Цикл
    Дв = Движения.ОстаткиТоваров.Добавить();
    Дв.ВидДвижения = "Приход";
    Дв.Номенклатура = Стр.Номенклатура;
    Дв.Количество = Стр.Количество;
  КонецЦикла;
КонецПроцедуры`
	registry := runtime.NewRegistry()
	registry.Load(runtime.LoadOptions{
		Entities:  []*metadata.Entity{doc},
		Programs:  map[string]*ast.Program{"ПоступлениеТоваров": mustParse(t, onPostSrc)},
		Registers: []*metadata.Register{reg},
	})
	interp := interpreter.New()
	interp.LookupProc = registry.GetModuleProc
	s := &Server{store: db, reg: registry, interp: interp, lockMgr: runtime.NewLockManager(), messages: NewMessageStore()}
	dp := newDocsRoot(s, interpreter.NewTxState(ctx)).Get("ПоступлениеТоваров").(*docProxy)
	return ctx, db, s, dp, doc
}

// postOne создаёт+заполняет+проводит документ ПОС-001 (Тумбочка x100) и возвращает writer.
func postOne(t *testing.T, dp *docProxy) *docWriter {
	t.Helper()
	w := dp.CallMethod("создать", nil).(*docWriter)
	w.Set("Номер", "ПОС-001")
	tp := w.Get("Товары").(*tpProxy)
	r := tp.CallMethod("добавить", nil).(*interpreter.MapThis)
	r.Set("Номенклатура", "Тумбочка")
	r.Set("Количество", float64(100))
	w.CallMethod("провести", nil)
	return w
}

// DSL .Провести() помеченного документа должно отклоняться.
func TestDocWriterPost_BlockedWhenMarked(t *testing.T) {
	ctx, db, _, dp, _ := newPostingDoc(t)
	w := postOne(t, dp)
	if err := db.MarkForDeletion(ctx, "ПоступлениеТоваров", w.obj.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := w.post(); !errors.Is(err, storage.ErrPostingDeletionMarked) {
		t.Fatalf("ожидался ErrPostingDeletionMarked, получили %v", err)
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `go test ./internal/ui/ -run TestDocWriterPost_BlockedWhenMarked`
Expected: FAIL — `w.post()` всё ещё проводит помеченный документ (guard'а нет), `err == nil`.

- [ ] **Step 3: Добавить guard в `docWriter.post` + import storage**

In `internal/ui/dsl_documents.go`, заменить блок импортов (строки 3-13):

```go
import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/boffik/onebase/internal/dsl/interpreter"
	"github.com/boffik/onebase/internal/entityservice"
	"github.com/boffik/onebase/internal/metadata"
	"github.com/boffik/onebase/internal/runtime"
	"github.com/boffik/onebase/internal/storage"
)
```

И в `post()` (строки 428-430) заменить:

```go
func (w *docWriter) post() error {
	ctx := w.ctx()
	w.ensureSelfRef()
```

на:

```go
func (w *docWriter) post() error {
	ctx := w.ctx()
	// Инвариант: помеченный на удаление документ нельзя провести (как в 1С).
	if marked, err := w.s.store.IsMarkedForDeletion(ctx, w.entity.Name, w.obj.ID); err != nil {
		return err
	} else if marked {
		return storage.ErrPostingDeletionMarked
	}
	w.ensureSelfRef()
```

- [ ] **Step 4: Добавить guard в HTTP `postDocument`**

In `internal/ui/handlers.go`, после загрузки документа (строки 900-904):

```go
	row, err := s.store.GetByID(r.Context(), entity.Name, id, entity)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
```

добавить сразу следом:

```go
	if asBool(row["deletion_mark"]) {
		// Помеченный на удаление документ проводить нельзя.
		http.Redirect(w, r,
			"/ui/"+strings.ToLower(string(entity.Kind))+"/"+entity.Name+"/"+id.String()+
				"?posting_error="+url.QueryEscape("Документ помечен на удаление: проведение невозможно"),
			http.StatusSeeOther)
		return
	}
```

- [ ] **Step 5: Запустить тест — убедиться, что проходит, и весь пакет компилируется**

Run: `go test ./internal/ui/ -run TestDocWriterPost_BlockedWhenMarked -v`
Expected: PASS

- [ ] **Step 6: Коммит**

```bash
git add internal/ui/handlers.go internal/ui/dsl_documents.go internal/ui/deletion_posting_test.go
git commit -m "feat(ui): запрет проведения помеченного — HTTP и DSL-пути (#36)"
```

---

## Task 4: ui — `markForDeletion` (авто-отмена проведения) + правка `deleteRecord` (пометка/снятие)

**Files:**
- Modify: `internal/ui/handlers.go` (новый метод рядом с `clearMovements`/`deleteRecord`; правка `deleteRecord:1006-1016`)
- Modify: `internal/ui/handlers.go:722-725` (добавить `vals["deletion_mark"]`)
- Test: `internal/ui/deletion_posting_test.go` (добавить функцию)

- [ ] **Step 1: Добавить падающий тест в `deletion_posting_test.go`**

Дописать в конец `internal/ui/deletion_posting_test.go`:

```go
// Пометка проведённого документа на удаление авто-отменяет проведение
// (чистит движения, снимает posted). Снятие пометки проведение не возвращает.
func TestServerMarkForDeletion_AutoUnposts(t *testing.T) {
	ctx, db, s, dp, doc := newPostingDoc(t)
	w := postOne(t, dp)

	var mov int
	db.QueryRow(ctx, "SELECT COUNT(*) FROM рег_остаткитоваров").Scan(&mov)
	if mov != 1 {
		t.Fatalf("до пометки ожидалось 1 движение, получили %d", mov)
	}

	if err := s.markForDeletion(ctx, doc, w.obj.ID, true); err != nil {
		t.Fatal(err)
	}
	db.QueryRow(ctx, "SELECT COUNT(*) FROM рег_остаткитоваров").Scan(&mov)
	if mov != 0 {
		t.Errorf("после пометки движений должно быть 0, получили %d", mov)
	}
	var posted, marked bool
	db.QueryRow(ctx, "SELECT posted, deletion_mark FROM поступлениетоваров LIMIT 1").Scan(&posted, &marked)
	if posted {
		t.Error("проведение должно быть снято при пометке")
	}
	if !marked {
		t.Error("документ должен быть помечен на удаление")
	}

	// Снятие пометки проведение НЕ возвращает.
	if err := s.markForDeletion(ctx, doc, w.obj.ID, false); err != nil {
		t.Fatal(err)
	}
	db.QueryRow(ctx, "SELECT posted, deletion_mark FROM поступлениетоваров LIMIT 1").Scan(&posted, &marked)
	if posted {
		t.Error("снятие пометки не должно проводить документ")
	}
	if marked {
		t.Error("пометка должна быть снята")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что не компилируется/падает**

Run: `go test ./internal/ui/ -run TestServerMarkForDeletion_AutoUnposts`
Expected: FAIL — `s.markForDeletion` undefined.

- [ ] **Step 3: Добавить метод `markForDeletion`**

In `internal/ui/handlers.go`, добавить перед `unpostDocument` (т.е. сразу после `clearMovements`, после строки 960):

```go
// markForDeletion помечает/снимает пометку на удаление. При пометке проведённого
// документа сперва отменяет проведение (чистит движения по всем регистрам и
// снимает posted) — пометка и проведённость взаимоисключающи (как в 1С). Снятие
// пометки проведение НЕ возвращает. Транзакцию метод не открывает: HTTP-вызовы
// оборачивают его в store.WithTx, DSL-путь использует живой ctx (как DeleteRef).
func (s *Server) markForDeletion(ctx context.Context, entity *metadata.Entity, id uuid.UUID, mark bool) error {
	if mark && entity.Posting {
		row, err := s.store.GetByID(ctx, entity.Name, id, entity)
		if err != nil {
			return err
		}
		if asBool(row["posted"]) {
			if err := s.clearMovements(ctx, entity.Name, id); err != nil {
				return err
			}
			if err := s.store.SetPosted(ctx, entity.Name, id, false); err != nil {
				return err
			}
		}
	}
	return s.store.MarkForDeletion(ctx, entity.Name, id, mark)
}
```

- [ ] **Step 4: Перевести `deleteRecord` на `markForDeletion` + ветку `mark=0`**

In `internal/ui/handlers.go`, заменить строки 1006-1016:

```go
	markOnly := r.URL.Query().Get("mark") == "1"

	if !isAdmin || markOnly {
		// Non-admin or explicit mark-only: mark for deletion
		if err := s.store.MarkForDeletion(r.Context(), entity.Name, id, true); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.Redirect(w, r, listURL(entity), http.StatusSeeOther)
		return
	}
```

на:

```go
	markParam := r.URL.Query().Get("mark")

	// Снятие пометки на удаление (mark=0) — без возврата проведения.
	if markParam == "0" {
		if err := s.store.MarkForDeletion(r.Context(), entity.Name, id, false); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.Redirect(w, r, listURL(entity), http.StatusSeeOther)
		return
	}

	if !isAdmin || markParam == "1" {
		// Non-admin или явная пометка: пометить на удаление с авто-отменой
		// проведения для проведённого документа (в одной транзакции).
		if err := s.store.WithTx(r.Context(), func(ctx context.Context) error {
			return s.markForDeletion(ctx, entity, id, true)
		}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.Redirect(w, r, listURL(entity), http.StatusSeeOther)
		return
	}
```

- [ ] **Step 5: Добавить `vals["deletion_mark"]` в карточку документа**

In `internal/ui/handlers.go`, заменить строки 722-725:

```go
	// Include posted status for documents
	if entity.Kind == metadata.KindDocument {
		vals["posted"] = fmt.Sprintf("%v", row["posted"])
	}
```

на:

```go
	// Include posted status + deletion mark for documents
	if entity.Kind == metadata.KindDocument {
		vals["posted"] = fmt.Sprintf("%v", row["posted"])
		vals["deletion_mark"] = fmt.Sprintf("%v", row["deletion_mark"])
	}
```

- [ ] **Step 6: Запустить тест — убедиться, что проходит**

Run: `go test ./internal/ui/ -run TestServerMarkForDeletion_AutoUnposts -v`
Expected: PASS

- [ ] **Step 7: Коммит**

```bash
git add internal/ui/handlers.go internal/ui/deletion_posting_test.go
git commit -m "feat(ui): пометка проведённого авто-отменяет проведение; снятие пометки (#36)"
```

---

## Task 5: ui — DSL-методы `ОтменитьПроведение` / `ПометитьНаУдаление` / `СнятьПометку`

**Files:**
- Modify: `internal/ui/dsl_documents.go` (кейсы в `docProxy.CallMethod:79-120`; хелперы рядом с `DeleteRef:153-165`)
- Test: `internal/ui/deletion_posting_test.go` (добавить функцию)

- [ ] **Step 1: Добавить падающий тест**

Дописать в конец `internal/ui/deletion_posting_test.go`:

```go
// DSL-методы менеджера документов: ОтменитьПроведение / ПометитьНаУдаление / СнятьПометку.
func TestDocsRoot_UnpostAndMarkMethods(t *testing.T) {
	ctx, db, _, dp, _ := newPostingDoc(t)
	_ = postOne(t, dp)
	ref := dp.CallMethod("найтипономеру", []any{"ПОС-001"}).(*interpreter.Ref)

	// ОтменитьПроведение → движения 0, posted false.
	dp.CallMethod("отменитьпроведение", []any{ref})
	var mov int
	db.QueryRow(ctx, "SELECT COUNT(*) FROM рег_остаткитоваров").Scan(&mov)
	if mov != 0 {
		t.Errorf("после ОтменитьПроведение движений 0 ожидалось, получили %d", mov)
	}
	var posted bool
	db.QueryRow(ctx, "SELECT posted FROM поступлениетоваров LIMIT 1").Scan(&posted)
	if posted {
		t.Error("posted=false ожидался после ОтменитьПроведение")
	}

	// ПометитьНаУдаление → deletion_mark true.
	dp.CallMethod("пометитьнаудаление", []any{ref})
	var marked bool
	db.QueryRow(ctx, "SELECT deletion_mark FROM поступлениетоваров LIMIT 1").Scan(&marked)
	if !marked {
		t.Error("документ должен быть помечен после ПометитьНаУдаление")
	}

	// СнятьПометку → deletion_mark false.
	dp.CallMethod("снятьпометку", []any{ref})
	db.QueryRow(ctx, "SELECT deletion_mark FROM поступлениетоваров LIMIT 1").Scan(&marked)
	if marked {
		t.Error("пометка должна быть снята после СнятьПометку")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `go test ./internal/ui/ -run TestDocsRoot_UnpostAndMarkMethods`
Expected: FAIL — методы не распознаны (`CallMethod` вернёт nil → паника на `.(*interpreter.Ref)` отсутствует, но `отменитьпроведение` вернёт nil и движения не очистятся → ассерт `mov != 0`).

- [ ] **Step 3: Добавить кейсы методов в `docProxy.CallMethod`**

In `internal/ui/dsl_documents.go`, в `CallMethod` после кейса `"удалить", "delete"` (после строки 119, перед комментарием `// Fallback на модуль менеджера`) добавить:

```go
	case "отменитьпроведение", "unpost":
		if len(args) == 0 {
			interpreter.RaiseUserError("ОтменитьПроведение(" + p.entity.Name + "): не передана ссылка")
		}
		ref, ok := args[0].(*interpreter.Ref)
		if !ok {
			interpreter.RaiseUserError(fmt.Sprintf("ОтменитьПроведение(%s): ожидается ссылка, получено %T", p.entity.Name, args[0]))
		}
		if err := p.unpostRef(ref.UUID); err != nil {
			interpreter.RaiseUserError("ОтменитьПроведение(" + p.entity.Name + "): " + err.Error())
		}
		return nil
	case "пометитьнаудаление", "markfordeletion":
		if len(args) == 0 {
			interpreter.RaiseUserError("ПометитьНаУдаление(" + p.entity.Name + "): не передана ссылка")
		}
		ref, ok := args[0].(*interpreter.Ref)
		if !ok {
			interpreter.RaiseUserError(fmt.Sprintf("ПометитьНаУдаление(%s): ожидается ссылка, получено %T", p.entity.Name, args[0]))
		}
		if err := p.markRef(ref.UUID, true); err != nil {
			interpreter.RaiseUserError("ПометитьНаУдаление(" + p.entity.Name + "): " + err.Error())
		}
		return nil
	case "снятьпометку", "unmarkdeletion":
		if len(args) == 0 {
			interpreter.RaiseUserError("СнятьПометку(" + p.entity.Name + "): не передана ссылка")
		}
		ref, ok := args[0].(*interpreter.Ref)
		if !ok {
			interpreter.RaiseUserError(fmt.Sprintf("СнятьПометку(%s): ожидается ссылка, получено %T", p.entity.Name, args[0]))
		}
		if err := p.markRef(ref.UUID, false); err != nil {
			interpreter.RaiseUserError("СнятьПометку(" + p.entity.Name + "): " + err.Error())
		}
		return nil
```

- [ ] **Step 4: Добавить хелперы `unpostRef` / `markRef`**

In `internal/ui/dsl_documents.go`, после `DeleteRef` (после строки 165) добавить:

```go
// unpostRef отменяет проведение документа: чистит движения по всем регистрам и
// снимает posted (аналог UI-хендлера unpostDocument). Использует живой ctx (как
// DeleteRef) — участвует в открытой DSL-транзакции, если она есть.
func (p *docProxy) unpostRef(uuidStr string) error {
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		return fmt.Errorf("неверный идентификатор ссылки: %q", uuidStr)
	}
	ctx := p.ctx()
	if p.entity.Posting {
		if err := p.s.clearMovements(ctx, p.entity.Name, id); err != nil {
			return fmt.Errorf("очистка движений: %w", err)
		}
	}
	return p.s.store.SetPosted(ctx, p.entity.Name, id, false)
}

// markRef помечает/снимает пометку на удаление (с авто-отменой проведения при
// пометке проведённого документа). Использует живой ctx.
func (p *docProxy) markRef(uuidStr string, mark bool) error {
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		return fmt.Errorf("неверный идентификатор ссылки: %q", uuidStr)
	}
	return p.s.markForDeletion(p.ctx(), p.entity, id, mark)
}
```

- [ ] **Step 5: Запустить тест — убедиться, что проходит**

Run: `go test ./internal/ui/ -run TestDocsRoot_UnpostAndMarkMethods -v`
Expected: PASS

- [ ] **Step 6: Коммит**

```bash
git add internal/ui/dsl_documents.go internal/ui/deletion_posting_test.go
git commit -m "feat(ui): DSL ОтменитьПроведение/ПометитьНаУдаление/СнятьПометку (#36)"
```

---

## Task 6: шаблоны — скрыть «Провести» у помеченного, кнопка «Снять пометку», списочные действия

Шаблоны в этом проекте не покрыты юнит-тестами — проверяем сборкой и ручной верификацией (раздел Verification). Строки оборачиваем в `t $.Lang "…"` (i18ncheck — предупреждение, не блокирует).

**Files:**
- Modify: `internal/ui/templates.go:1025-1033` (скрыть post-кнопки), `:1033-1034` (кнопка снятия пометки), `:868-877` (data-атрибуты строки), `:913-915` (`_canUnpost`), `:963-967` (пункты меню)
- Modify: `internal/ui/templates_managed.go:331-339`

- [ ] **Step 1: Autogen-форма — скрыть «Провести»/«Провести и закрыть» у помеченного**

In `internal/ui/templates.go`, заменить строки 1025-1027:

```go
  {{if .Entity.Posting}}
    {{if $.CanPost}}<button class="btn btn-primary" type="submit" name="_action" value="post" form="main-form">{{t $.Lang "Провести"}}</button>{{end}}
    {{if $.CanPost}}<button class="btn btn-post" type="submit" name="_action" value="post_and_close" form="main-form">{{t $.Lang "Провести и закрыть"}}</button>{{end}}
```

на:

```go
  {{if .Entity.Posting}}
    {{if ne (index .Values "deletion_mark") "true"}}
      {{if $.CanPost}}<button class="btn btn-primary" type="submit" name="_action" value="post" form="main-form">{{t $.Lang "Провести"}}</button>{{end}}
      {{if $.CanPost}}<button class="btn btn-post" type="submit" name="_action" value="post_and_close" form="main-form">{{t $.Lang "Провести и закрыть"}}</button>{{end}}
    {{end}}
```

- [ ] **Step 2: Autogen-форма — кнопка «Снять пометку на удаление»**

In `internal/ui/templates.go`, после строки 1033 (закрывающий `{{end}}` блока `{{if .Entity.Posting}}`) и перед `{{if not .IsNew}}` (строка 1034) вставить:

```go
  {{if and .CanDelete (not .IsNew) (eq (index .Values "deletion_mark") "true")}}
    <form method="POST" action="/ui/{{lower (str .Entity.Kind)}}/{{.Entity.Name}}/{{.ID}}/delete?mark=0" style="display:inline">
      <button class="btn btn-sm btn-secondary" type="submit">{{t $.Lang "Снять пометку на удаление"}}</button>
    </form>
  {{end}}
```

- [ ] **Step 3: Managed-форма — те же правки**

In `internal/ui/templates_managed.go`, заменить строки 331-339:

```go
  {{if .Entity.Posting}}
    {{if .CanPost}}<button class="btn btn-primary" type="submit" name="_action" value="post" form="main-form">Провести</button>{{end}}
    {{if .CanPost}}<button class="btn btn-post" type="submit" name="_action" value="post_and_close" form="main-form">Провести и закрыть</button>{{end}}
    {{if not .IsNew}}
      {{if eq (index .Values "posted") "true"}}
        {{if $.CanUnpost}}<button class="btn btn-sm" style="background:#e2e8f0;color:#374151" form="form-unpost" type="submit">Отменить проведение</button>{{end}}
      {{end}}
    {{end}}
  {{end}}
```

на:

```go
  {{if .Entity.Posting}}
    {{if ne (index .Values "deletion_mark") "true"}}
      {{if .CanPost}}<button class="btn btn-primary" type="submit" name="_action" value="post" form="main-form">Провести</button>{{end}}
      {{if .CanPost}}<button class="btn btn-post" type="submit" name="_action" value="post_and_close" form="main-form">Провести и закрыть</button>{{end}}
    {{end}}
    {{if not .IsNew}}
      {{if eq (index .Values "posted") "true"}}
        {{if $.CanUnpost}}<button class="btn btn-sm" style="background:#e2e8f0;color:#374151" form="form-unpost" type="submit">Отменить проведение</button>{{end}}
      {{end}}
    {{end}}
  {{end}}
  {{if and .CanDelete (not .IsNew) (eq (index .Values "deletion_mark") "true")}}
    <form method="POST" action="/ui/{{lower (str .Entity.Kind)}}/{{.Entity.Name}}/{{.ID}}/delete?mark=0" style="display:inline">
      <button class="btn btn-sm btn-secondary" type="submit">Снять пометку на удаление</button>
    </form>
  {{end}}
```

- [ ] **Step 4: Список — data-атрибуты строки (posted/marked + url'ы)**

In `internal/ui/templates.go`, после строки 876 (`data-del-url=...`) вставить (внутри открывающего тега `<tr ...>`, до `data-open-url`):

```go
  data-posted="{{if index $row "posted"}}1{{end}}"
  data-marked="{{if index $row "deletion_mark"}}1{{end}}"
  data-unpost-url="/ui/{{lower (str $.Entity.Kind)}}/{{lower $.Entity.Name}}/{{index $row "id"}}/unpost"
  data-unmark-url="/ui/{{lower (str $.Entity.Kind)}}/{{lower $.Entity.Name}}/{{index $row "id"}}/delete?mark=0"
```

- [ ] **Step 5: Список — JS-флаг `_canUnpost`**

In `internal/ui/templates.go`, после строки 914 (`var _canDelete=...;`) вставить:

```go
var _canUnpost={{if .CanUnpost}}true{{else}}false{{end}};
```

- [ ] **Step 6: Список — пункты контекстного меню**

In `internal/ui/templates.go`, в `listCtxMenu`, после блока `_canDelete` (после строки 966) и перед строкой 967 (`if(_isAdmin&&!isPredefined)...`) вставить:

```go
  if(_canUnpost&&tr.dataset.posted==='1')items.push({label:'{{t $.Lang "Отменить проведение"}}',fn:function(){listSubmit(tr.dataset.unpostUrl,'{{t $.Lang "Отменить проведение?"}}');}});
  if(_canDelete&&tr.dataset.marked==='1'&&!isPredefined)items.push({label:'{{t $.Lang "Снять пометку на удаление"}}',fn:function(){listSubmit(tr.dataset.unmarkUrl,'{{t $.Lang "Снять пометку на удаление?"}}');}});
```

- [ ] **Step 7: Собрать и прогнать пакет**

Run: `go build ./... && go test ./internal/ui/`
Expected: успешная сборка; тесты пакета `ui` зелёные (шаблоны парсятся при инициализации — синтаксическая ошибка в шаблоне уронила бы тесты).

- [ ] **Step 8: Коммит**

```bash
git add internal/ui/templates.go internal/ui/templates_managed.go
git commit -m "feat(ui): скрыть «Провести» у помеченного; списочные отмена проведения/снятие пометки (#36)"
```

---

## Task 7: финальная верификация и закрытие плана

**Files:**
- Modify: `Plans/50-deletion-mark-posting.md` (статус), `Plans/README.md` (строка плана)

- [ ] **Step 1: Полный прогон тестов и сборки**

Run: `go build ./... && go test ./...`
Expected: PASS по всем пакетам.

- [ ] **Step 2: Валидация примера конфигурации**

Run: `go build -o onebase.exe ./cmd/onebase && ./onebase.exe check --project examples/trade`
Expected: валидация без ошибок (компиляция запросов модулей проходит).

- [ ] **Step 3: Ручная верификация (end-to-end)**

```bash
./onebase.exe migrate --project examples/trade --sqlite /tmp/t36.db
./onebase.exe run --project examples/trade --sqlite /tmp/t36.db --port 8080
```
Проверить (по разделу Verification спеки):
1. Создать и провести документ → появились движения.
2. В списке ПКМ по проведённому → «Отменить проведение» → движения исчезли, значок «✓» снят.
3. Провести снова; в списке «Пометить на удаление» → строка зачёркнута, движения исчезли, «✓» снят.
4. На карточке помеченного документа кнопок «Провести»/«Провести и закрыть» нет; есть «Снять пометку на удаление».
5. POST `/ui/document/<doc>/<id>/post` напрямую (curl) для помеченного → редирект с `posting_error`.
6. ПКМ → «Снять пометку на удаление» → зачёркивание ушло, документ остаётся непроведённым.

- [ ] **Step 4: Обновить статус плана**

In `Plans/50-deletion-mark-posting.md` заменить строку `**Статус:** проектирование` на `**Статус:** ✅ Реализовано`.

В `Plans/README.md` добавить строку в раздел «Реализованные этапы» (после строки про этап 38):

```markdown
| 50 | [50-deletion-mark-posting.md](50-deletion-mark-posting.md) | Пометка на удаление ↔ проведение (issue #36) | ✅ Реализовано |
```

- [ ] **Step 5: Финальный коммит**

```bash
git add Plans/50-deletion-mark-posting.md Plans/README.md
git commit -m "docs(36): план 50 реализован — пометка на удаление ↔ проведение"
```

---

## Self-Review

**Spec coverage:**
- Симптом 1 (движения остаются) → Task 4 (`markForDeletion` авто-отмена) + тест `TestServerMarkForDeletion_AutoUnposts`. ✓
- Симптом 3 (помеченный проводится) → Task 1 (backstop), Task 2 (entityservice), Task 3 (HTTP + DSL) + тесты во всех трёх. ✓
- Симптом 2 (отмена проведения) → форма уже есть; Task 5 (DSL `ОтменитьПроведение`), Task 6 (списочный пункт). ✓
- «Снять пометку» → Task 4 (ветка `mark=0`), Task 5 (DSL `СнятьПометку`), Task 6 (кнопка формы + пункт списка). ✓
- query/, схема БД, миграция данных — спекой исключены, тасков нет (корректно). ✓

**Placeholder scan:** плейсхолдеров нет — каждый шаг содержит полный код и точные команды.

**Type consistency:**
- `ErrPostingDeletionMarked` (storage) — Task 1 определяет, Tasks 2/3 используют `storage.ErrPostingDeletionMarked`. ✓
- `IsMarkedForDeletion(ctx, name, id) (bool, error)` — единая сигнатура в Tasks 1/2/3. ✓
- `(*Server).markForDeletion(ctx, entity, id, mark)` — Task 4 определяет, Task 5 (`markRef`) и `deleteRecord` используют ту же сигнатуру. ✓
- DSL-кейсы регистронезависимы (`strings.ToLower(method)`), значения в тестах — нижний регистр (`"отменитьпроведение"` и т.д.). ✓
- Тест-хелперы `newPostingDoc`/`postOne` определены в Task 3, переиспользуются в Tasks 4/5 (тот же файл `internal/ui/deletion_posting_test.go`). ✓
