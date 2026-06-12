package interpreter

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/i18n/i18nerr"
	"github.com/ivantit66/onebase/internal/metadata"
)

// CatalogsDB extends PredefinedDB with field-based lookups and writes.
// Returns ("", "", false, nil) on not-found so the DSL can compare against nil.
type CatalogsDB interface {
	PredefinedDB
	FindCatalogByField(ctx context.Context, entity *metadata.Entity, fieldName, value string) (idStr, display string, ok bool, err error)
	// WriteCatalogRecord upserts a record. idStr пустой →
	// генерируется новый UUID. Возвращает UUID записанной записи.
	WriteCatalogRecord(ctx context.Context, entity *metadata.Entity, idStr string, fields map[string]any) (string, error)
	// Delete удаляет запись справочника/документа по идентификатору.
	Delete(ctx context.Context, entityName string, id uuid.UUID) error
	// GetByID загружает запись по UUID. Возвращает поля шапки (включая
	// id, _version, deletion_mark и т.д.). Используется Ссылка.ПолучитьОбъект()
	// для редактирования существующих записей справочников.
	GetByID(ctx context.Context, entityName string, id uuid.UUID, entity *metadata.Entity) (map[string]any, error)
}

// EntityLookup resolves an entity name (case-insensitive) to its metadata.
type EntityLookup interface {
	GetEntity(name string) *metadata.Entity
}

// ManagerCaller — необязательный «вызыватель» процедур модуля менеджера
// (X.manager.os). Опционально цепляется к CatalogsRoot через
// WithManagerCaller — если не задан, CatalogProxy остаётся прежним.
//
// Семантика found: процедура была найдена в модуле менеджера. Если false —
// proxy продолжает обработку (например, возвращает nil как раньше).
type ManagerCaller interface {
	CallManager(entityName, method string, args []any) (result any, found bool, err error)
}

// CtxSource предоставляет «живой» контекст. Для обычного запуска это
// статический контекст; при активной DSL-транзакции — *TxState, чей
// Ctx() несёт открытую транзакцию — запись справочника
// из обработки участвует в ней.
type CtxSource interface {
	Ctx() context.Context
}

// staticCtx — CtxSource c фиксированным контекстом (нет транзакции).
type staticCtx struct{ ctx context.Context }

func (s staticCtx) Ctx() context.Context { return s.ctx }

// NewStaticCtx wraps a plain context as a CtxSource.
func NewStaticCtx(ctx context.Context) CtxSource { return staticCtx{ctx: ctx} }

// CatalogsRoot is the DSL global Справочники / Catalogs.
type CatalogsRoot struct {
	db     CatalogsDB
	lookup EntityLookup
	ctxSrc CtxSource
	caller ManagerCaller // optional — fallback к модулю менеджера в CallMethod
}

// NewCatalogsRoot creates the root object for injection as DSL extraVar.
// ctxSrc — источник живого контекста (staticCtx или *TxState).
func NewCatalogsRoot(ctxSrc CtxSource, db CatalogsDB, lookup EntityLookup) *CatalogsRoot {
	return &CatalogsRoot{db: db, lookup: lookup, ctxSrc: ctxSrc}
}

// WithManagerCaller подключает обработчик пользовательских методов
// модуля менеджера. Возвращает себя для цепочки.
func (r *CatalogsRoot) WithManagerCaller(c ManagerCaller) *CatalogsRoot {
	r.caller = c
	return r
}

func (r *CatalogsRoot) Get(entityName string) any {
	entity := r.lookup.GetEntity(entityName)
	if entity == nil {
		return nil
	}
	return &CatalogProxy{entity: entity, db: r.db, ctxSrc: r.ctxSrc, caller: r.caller}
}

func (r *CatalogsRoot) Set(_ string, _ any) {}

// CatalogProxy resolves predefined items, runtime lookups, and record creation.
//
//	Справочники.ТипЦен.Закупочная                  → *Ref to predefined item
//	Справочники.ТипЦен.НайтиПоНаименованию("X")     → *Ref or nil
//	Справочники.Контрагент.Создать()                → *CatalogRecordWriter
type CatalogProxy struct {
	entity *metadata.Entity
	db     CatalogsDB
	ctxSrc CtxSource
	caller ManagerCaller // optional — для вызовов методов модуля менеджера
}

// NewCatalogProxy создаёт менеджера справочника для привязки к ссылкам,
// приходящим из БД (см. enrichHeaderRefs/enrichTPRowsWithRefs в ui).
// Так Ссылка.Удалить()/ПолучитьОбъект() работают на ссылках реквизитов
// шапки/ТЧ, а не только на ссылках, созданных через Справочники.X.НайтиПо…
func NewCatalogProxy(entity *metadata.Entity, db CatalogsDB, ctxSrc CtxSource) *CatalogProxy {
	return &CatalogProxy{entity: entity, db: db, ctxSrc: ctxSrc}
}

func (p *CatalogProxy) ctx() context.Context {
	if p.ctxSrc != nil {
		return p.ctxSrc.Ctx()
	}
	return context.Background()
}

// Get is called for foo.Bar attribute access — predefined item lookup.
func (p *CatalogProxy) Get(itemName string) any {
	for _, item := range p.entity.Predefined {
		if strings.EqualFold(item.Name, itemName) {
			id, err := p.db.GetPredefinedIDStr(p.ctx(), p.entity.Name, item.Name)
			if err != nil || id == "" {
				return nil
			}
			return &Ref{UUID: id, Name: item.Name, Type: p.entity.Name, Manager: p}
		}
	}
	return nil
}

func (p *CatalogProxy) Set(_ string, _ any) {}

// CallMethod implements MethodCallable for method-style invocation.
func (p *CatalogProxy) CallMethod(method string, args []any) any {
	switch strings.ToLower(method) {
	case "найтипонаименованию", "findbyname":
		return p.findByField("Наименование", args)
	case "найтипокоду", "findbycode":
		return p.findByField("Код", args)
	case "найтипореквизиту", "findbyattribute":
		if len(args) < 2 {
			RaiseUserError("НайтиПоРеквизиту(" + p.entity.Name + "): нужны имя реквизита и значение")
		}
		field, ok := args[0].(string)
		if !ok {
			RaiseUserError("НайтиПоРеквизиту(" + p.entity.Name + "): имя реквизита должно быть строкой")
		}
		return p.findByField(field, args[1:])
	case "создать", "create":
		return &CatalogRecordWriter{
			entity: p.entity,
			db:     p.db,
			ctxSrc: p.ctxSrc,
			fields: map[string]any{},
		}
	case "удалить", "delete":
		if len(args) == 0 {
			RaiseUserError("Удалить(" + p.entity.Name + "): не передана ссылка")
		}
		ref, ok := args[0].(*Ref)
		if !ok {
			RaiseUserError(fmt.Sprintf("Удалить(%s): ожидается ссылка, получено %T", p.entity.Name, args[0]))
		}
		if err := p.DeleteRef(ref.UUID); err != nil {
			RaiseUserError("Удалить(" + p.entity.Name + "): " + err.Error())
		}
		return nil
	}
	// Fallback на модуль менеджера: Справочники.X.МойМетод(…). Если caller не
	// подключён или процедура не объявлена — ведёт себя как раньше (nil).
	if p.caller != nil {
		if result, found, err := p.caller.CallManager(p.entity.Name, method, args); found {
			if err != nil {
				RaiseUserError(p.entity.Name + "." + method + ": " + err.Error())
			}
			return result
		}
	}
	return nil
}

// DeleteRef реализует RefManager — удаление записи справочника по UUID.
func (p *CatalogProxy) DeleteRef(uuidStr string) error {
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		return i18nerr.Errorf("неверный идентификатор ссылки: %q", uuidStr)
	}
	return p.db.Delete(p.ctx(), p.entity.Name, id)
}

// LoadObject реализует RefManager — загружает существующую запись справочника
// по UUID и возвращает CatalogRecordWriter с предзаполненными полями, так что
// Ссылка.ПолучитьОбъект().Поле = … → Записать() обновит запись по тому же id.
func (p *CatalogProxy) LoadObject(uuidStr string) (any, error) {
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		return nil, i18nerr.Errorf("неверный идентификатор ссылки: %q", uuidStr)
	}
	row, err := p.db.GetByID(p.ctx(), p.entity.Name, id, p.entity)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]any, len(row))
	for _, f := range p.entity.Fields {
		if v, ok := row[f.Name]; ok && v != nil {
			fields[strings.ToLower(f.Name)] = v
		}
	}
	return &CatalogRecordWriter{
		entity: p.entity,
		db:     p.db,
		ctxSrc: p.ctxSrc,
		idStr:  uuidStr,
		fields: fields,
	}, nil
}

func (p *CatalogProxy) findByField(field string, args []any) any {
	if len(args) == 0 {
		return nil
	}
	value, ok := args[0].(string)
	if !ok {
		if r, ok := args[0].(*Ref); ok {
			value = r.Name
		} else {
			return nil
		}
	}
	idStr, display, found, err := p.db.FindCatalogByField(p.ctx(), p.entity, field, value)
	if err != nil || !found {
		return nil
	}
	return &Ref{UUID: idStr, Name: display, Type: p.entity.Name, Manager: p}
}

// CatalogRecordWriter — записываемый объект справочника/документа,
// созданный через Справочники.X.Создать().
//
//	Зап = Справочники.Контрагент.Создать();
//	Зап.Наименование = "ООО Ромашка";
//	Зап.ИНН = "7701234567";
//	Ссыл = Зап.Записать();   // → *Ref на записанную запись
type CatalogRecordWriter struct {
	entity *metadata.Entity
	db     CatalogsDB
	ctxSrc CtxSource
	idStr  string
	fields map[string]any
}

func (w *CatalogRecordWriter) ctx() context.Context {
	if w.ctxSrc != nil {
		return w.ctxSrc.Ctx()
	}
	return context.Background()
}

// Get — чтение установленного значения поля (case-insensitive).
func (w *CatalogRecordWriter) Get(name string) any {
	low := strings.ToLower(name)
	for k, v := range w.fields {
		if strings.ToLower(k) == low {
			return v
		}
	}
	return nil
}

// Set — установка значения поля (Зап.Поле = значение).
func (w *CatalogRecordWriter) Set(name string, v any) {
	w.fields[strings.ToLower(name)] = v
}

// Fields — имена заполненных полей объекта. Позволяет использовать объект
// как источник в ЗаполнитьЗначенияСвойств(Приёмник, Объект).
func (w *CatalogRecordWriter) Fields() []string {
	names := make([]string, 0, len(w.fields))
	for k := range w.fields {
		names = append(names, k)
	}
	return names
}

// CallMethod — Записать() / УстановитьЗначение().
func (w *CatalogRecordWriter) CallMethod(method string, args []any) any {
	switch strings.ToLower(method) {
	case "записать", "write":
		id, err := w.db.WriteCatalogRecord(w.ctx(), w.entity, w.idStr, w.fields)
		if err != nil {
			RaiseUserError("Записать(" + w.entity.Name + "): " + err.Error())
		}
		w.idStr = id
		name := ""
		if v := w.Get("Наименование"); v != nil {
			name = fmt.Sprintf("%v", v)
		}
		return &Ref{
			UUID: id, Name: name, Type: w.entity.Name,
			Manager: &CatalogProxy{entity: w.entity, db: w.db, ctxSrc: w.ctxSrc},
		}
	case "установитьзначение", "setvalue":
		if len(args) >= 2 {
			if n, ok := args[0].(string); ok {
				w.Set(n, args[1])
			}
		}
	case "этоновый", "isnew":
		return w.idStr == ""
	case "прочитать", "read":
		w.read()
		return nil
	}
	return nil
}

// read перечитывает поля объекта из БД (Объект.Прочитать()). Требует, чтобы
// объект уже был записан (иначе нечего читать).
func (w *CatalogRecordWriter) read() {
	if w.idStr == "" {
		RaiseUserError("Прочитать(" + w.entity.Name + "): объект ещё не записан")
	}
	id, err := uuid.Parse(w.idStr)
	if err != nil {
		RaiseUserError("Прочитать(" + w.entity.Name + "): неверный идентификатор")
	}
	row, err := w.db.GetByID(w.ctx(), w.entity.Name, id, w.entity)
	if err != nil {
		RaiseUserError("Прочитать(" + w.entity.Name + "): " + err.Error())
	}
	w.fields = make(map[string]any, len(row))
	for _, f := range w.entity.Fields {
		if v, ok := row[f.Name]; ok && v != nil {
			w.fields[strings.ToLower(f.Name)] = v
		}
	}
}
