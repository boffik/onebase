package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/shopspring/decimal"
)

// objectAttributeValue реализует DSL-функцию ЗначениеРеквизитаОбъекта(Ссылка,
// "Реквизит") — единичный запрос к БД за значением реквизита по ссылке.
// Ссылка несёт только UUID и наименование, поэтому остальные реквизиты
// доступны только так. Ссылочный реквизит возвращается как ссылка (Ref) —
// чтобы работали цепочки ЗначениеРеквизитаОбъекта(ЗначениеРеквизитаОбъекта(…)).
// Пустая ссылка / отсутствующая запись → nil; неверное имя реквизита →
// ошибка (иначе вернулся бы тихий nil — та самая боль).
func (s *Server) objectAttributeValue(ctx context.Context, args []any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("ЗначениеРеквизитаОбъекта: ожидаются ссылка и имя реквизита")
	}
	if args[0] == nil {
		return nil, nil
	}
	ref, ok := args[0].(*interpreter.Ref)
	if !ok {
		return nil, nil // не ссылка (например, строка из формы) — пропускаем
	}
	attrName := strings.TrimSpace(fmt.Sprint(args[1]))
	if attrName == "" {
		return nil, fmt.Errorf("ЗначениеРеквизитаОбъекта: не указано имя реквизита")
	}
	if ref.UUID == "" {
		return nil, nil // пустая ссылка
	}
	if ref.Type == "" {
		return nil, fmt.Errorf("ЗначениеРеквизитаОбъекта: у ссылки %q не определён тип объекта", ref.Name)
	}
	entity := s.reg.GetEntity(ref.Type)
	if entity == nil {
		return nil, fmt.Errorf("ЗначениеРеквизитаОбъекта: неизвестный тип объекта %q", ref.Type)
	}
	var field *metadata.Field
	for i := range entity.Fields {
		if strings.EqualFold(entity.Fields[i].Name, attrName) {
			field = &entity.Fields[i]
			break
		}
	}
	if field == nil {
		return nil, fmt.Errorf("ЗначениеРеквизитаОбъекта: у объекта %s нет реквизита %q", ref.Type, attrName)
	}
	id, err := uuid.Parse(ref.UUID)
	if err != nil {
		return nil, fmt.Errorf("ЗначениеРеквизитаОбъекта: неверный идентификатор ссылки %q", ref.UUID)
	}
	row, err := s.store.GetByID(ctx, entity.Name, id, entity)
	if err != nil || row == nil {
		return nil, nil // запись не найдена
	}
	val := row[field.Name]
	if field.RefEntity != "" {
		return s.refFromValue(ctx, field.RefEntity, val), nil
	}
	return normalizeAttrValue(field.Type, val), nil
}

// refFromValue строит ссылку на запись справочника/документа по UUID,
// подставляя наименование. Пустое значение → nil.
func (s *Server) refFromValue(ctx context.Context, refEntityName string, raw any) any {
	if raw == nil {
		return nil
	}
	idStr := fmt.Sprint(raw)
	if up, ok := raw.(interface{ GetRefUUID() string }); ok {
		idStr = up.GetRefUUID()
	}
	if idStr == "" || idStr == "<nil>" {
		return nil
	}
	name := idStr
	var refEntity = s.reg.GetEntity(refEntityName)
	if refEntity != nil {
		if id, err := uuid.Parse(idStr); err == nil {
			if rr, err := s.store.GetByID(ctx, refEntity.Name, id, refEntity); err == nil && rr != nil {
				name = firstStringField(rr, refEntity)
			}
		}
	}
	return &interpreter.Ref{
		UUID:    idStr,
		Name:    name,
		Type:    refEntityName,
		Manager: s.refManagerFor(refEntity, ctx),
	}
}

// normalizeAttrValue приводит значение реквизита к типу DSL независимо от
// бэкенда: SQLite хранит числа и булево как текст, PostgreSQL — нативно.
func normalizeAttrValue(ft metadata.FieldType, v any) any {
	if v == nil {
		return nil
	}
	switch ft {
	case metadata.FieldTypeNumber:
		switch n := v.(type) {
		case decimal.Decimal:
			return n
		case float64:
			return decimal.NewFromFloat(n)
		case int64:
			return decimal.NewFromInt(n)
		case int:
			return decimal.NewFromInt(int64(n))
		case string:
			if d, err := decimal.NewFromString(strings.TrimSpace(n)); err == nil {
				return d
			}
		}
	case metadata.FieldTypeBool:
		switch b := v.(type) {
		case bool:
			return b
		case int64:
			return b != 0
		case string:
			return b == "true" || b == "1" || strings.EqualFold(b, "да")
		}
	}
	return v
}
