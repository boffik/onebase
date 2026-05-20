package interpreter

import (
	"context"
	"errors"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

// fakeCatalogsDB stubs storage for catalog/predefined lookups in tests.
type fakeCatalogsDB struct {
	predefinedID map[string]string                  // "Entity/Name" → uuid
	byField      map[string]map[string]struct{ ID, Display string }
}

func (f *fakeCatalogsDB) GetPredefinedIDStr(_ context.Context, entityName, name string) (string, error) {
	if id, ok := f.predefinedID[entityName+"/"+name]; ok {
		return id, nil
	}
	return "", errors.New("not found")
}

func (f *fakeCatalogsDB) FindCatalogByField(_ context.Context, entity *metadata.Entity, fieldName, value string) (string, string, bool, error) {
	key := entity.Name + "/" + fieldName
	if rows, ok := f.byField[key]; ok {
		if hit, ok := rows[value]; ok {
			return hit.ID, hit.Display, true, nil
		}
	}
	return "", "", false, nil
}

type fakeEntityLookup struct{ m map[string]*metadata.Entity }

func (f *fakeEntityLookup) GetEntity(name string) *metadata.Entity {
	if e, ok := f.m[name]; ok {
		return e
	}
	return nil
}

func newCatalogsTestEnv() (*CatalogsRoot, *fakeCatalogsDB, *fakeEntityLookup) {
	entity := &metadata.Entity{
		Name:   "ТипЦен",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
		Predefined: []*metadata.PredefinedItem{
			{Name: "Закупочная"},
		},
	}
	db := &fakeCatalogsDB{
		predefinedID: map[string]string{
			"ТипЦен/Закупочная": "11111111-1111-1111-1111-111111111111",
		},
		byField: map[string]map[string]struct{ ID, Display string }{
			"ТипЦен/Наименование": {
				"Розничная": {ID: "22222222-2222-2222-2222-222222222222", Display: "Розничная"},
			},
		},
	}
	lookup := &fakeEntityLookup{m: map[string]*metadata.Entity{"ТипЦен": entity}}
	return NewCatalogsRoot(context.Background(), db, lookup), db, lookup
}

// Справочники.X.ИмяПредопределённой должно возвращать Ref.
func TestCatalogProxy_PredefinedAccess(t *testing.T) {
	root, _, _ := newCatalogsTestEnv()
	proxy := root.Get("ТипЦен")
	if proxy == nil {
		t.Fatal("Справочники.ТипЦен → nil, ожидался proxy")
	}
	cp, ok := proxy.(*CatalogProxy)
	if !ok {
		t.Fatalf("ожидался *CatalogProxy, получили %T", proxy)
	}
	v := cp.Get("Закупочная")
	ref, ok := v.(*Ref)
	if !ok {
		t.Fatalf("Закупочная → %T, ожидался *Ref", v)
	}
	if ref.UUID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("неверный UUID: %s", ref.UUID)
	}
	if ref.Name != "Закупочная" {
		t.Errorf("неверное имя: %s", ref.Name)
	}
}

func TestCatalogProxy_PredefinedCaseInsensitive(t *testing.T) {
	root, _, _ := newCatalogsTestEnv()
	cp := root.Get("ТипЦен").(*CatalogProxy)
	if v := cp.Get("закупочная"); v == nil {
		t.Errorf("lowercase предопределённого не нашёлся")
	}
}

func TestCatalogProxy_PredefinedMissing(t *testing.T) {
	root, _, _ := newCatalogsTestEnv()
	cp := root.Get("ТипЦен").(*CatalogProxy)
	if v := cp.Get("НетТакого"); v != nil {
		t.Errorf("ожидался nil, получили %v", v)
	}
}

// НайтиПоНаименованию должно искать в catalog по полю Наименование.
func TestCatalogProxy_FindByName_Found(t *testing.T) {
	root, _, _ := newCatalogsTestEnv()
	cp := root.Get("ТипЦен").(*CatalogProxy)
	v := cp.CallMethod("найтипонаименованию", []any{"Розничная"})
	ref, ok := v.(*Ref)
	if !ok {
		t.Fatalf("ожидался *Ref, получили %T", v)
	}
	if ref.UUID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("неверный UUID: %s", ref.UUID)
	}
}

func TestCatalogProxy_FindByName_NotFound(t *testing.T) {
	root, _, _ := newCatalogsTestEnv()
	cp := root.Get("ТипЦен").(*CatalogProxy)
	if v := cp.CallMethod("найтипонаименованию", []any{"НетТакого"}); v != nil {
		t.Errorf("ожидался nil, получили %v", v)
	}
}

func TestCatalogProxy_UnknownEntity(t *testing.T) {
	root, _, _ := newCatalogsTestEnv()
	if v := root.Get("НетТакогоСправочника"); v != nil {
		t.Errorf("Справочники.НетТакого → %v, ожидался nil", v)
	}
}
