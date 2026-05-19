package interpreter

import (
	"context"
	"strings"

	"github.com/ivantit66/onebase/internal/metadata"
)

// CatalogsDB extends PredefinedDB with field-based lookups for runtime search.
// Returns ("", "", false, nil) on not-found so the DSL can compare against nil.
type CatalogsDB interface {
	PredefinedDB
	FindCatalogByField(ctx context.Context, entity *metadata.Entity, fieldName, value string) (idStr, display string, ok bool, err error)
}

// EntityLookup resolves an entity name (case-insensitive) to its metadata.
// Catalog access needs this to know what fields exist.
type EntityLookup interface {
	GetEntity(name string) *metadata.Entity
}

// CatalogsRoot is the DSL global Справочники / Catalogs.
// Each property access (.ТипЦен) returns a CatalogProxy for that entity.
type CatalogsRoot struct {
	db     CatalogsDB
	lookup EntityLookup
	ctx    context.Context
}

// NewCatalogsRoot creates the root object for injection as DSL extraVar.
func NewCatalogsRoot(ctx context.Context, db CatalogsDB, lookup EntityLookup) *CatalogsRoot {
	return &CatalogsRoot{db: db, lookup: lookup, ctx: ctx}
}

func (r *CatalogsRoot) Get(entityName string) any {
	entity := r.lookup.GetEntity(entityName)
	if entity == nil {
		return nil
	}
	return &CatalogProxy{entity: entity, db: r.db, ctx: r.ctx}
}

func (r *CatalogsRoot) Set(_ string, _ any) {}

// CatalogProxy resolves predefined items (by attr access) and exposes
// runtime lookups via methods.
//
//	Справочники.ТипЦен.Закупочная                 → *Ref to predefined item
//	Справочники.ТипЦен.НайтиПоНаименованию("X")    → *Ref or nil
type CatalogProxy struct {
	entity *metadata.Entity
	db     CatalogsDB
	ctx    context.Context
}

// Get is called for foo.Bar attribute access. Looks up a predefined item
// declared in the entity YAML.
func (p *CatalogProxy) Get(itemName string) any {
	// Walk declared predefined items for a case-insensitive name match.
	for _, item := range p.entity.Predefined {
		if strings.EqualFold(item.Name, itemName) {
			id, err := p.db.GetPredefinedIDStr(p.ctx, p.entity.Name, item.Name)
			if err != nil || id == "" {
				return nil
			}
			return &Ref{UUID: id, Name: item.Name}
		}
	}
	return nil
}

func (p *CatalogProxy) Set(_ string, _ any) {}

// CallMethod implements MethodCallable for method-style invocation.
func (p *CatalogProxy) CallMethod(method string, args []any) any {
	switch method {
	case "найтипонаименованию", "findbyname":
		return p.findByField("Наименование", args)
	case "найтипокоду", "findbycode":
		return p.findByField("Код", args)
	}
	return nil
}

func (p *CatalogProxy) findByField(field string, args []any) any {
	if len(args) == 0 {
		return nil
	}
	value, ok := args[0].(string)
	if !ok {
		// Allow *Ref or anything stringable — fall back to its display value.
		if r, ok := args[0].(*Ref); ok {
			value = r.Name
		} else {
			return nil
		}
	}
	idStr, display, found, err := p.db.FindCatalogByField(p.ctx, p.entity, field, value)
	if err != nil || !found {
		return nil
	}
	return &Ref{UUID: idStr, Name: display}
}

