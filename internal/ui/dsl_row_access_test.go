package ui

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
)

func dslRLSTestUser(login string, entity string, ops ...string) *auth.User {
	return &auth.User{Login: login, Roles: []*auth.Role{{
		Permissions: auth.Permission{
			Catalogs: map[string][]string{entity: ops},
			RowAccess: auth.RowAccess{Catalogs: map[string]auth.RowPolicies{
				entity: {"read": {Field: "Owner", Op: "eq", Value: auth.RowValue{User: "login"}}},
			}},
		},
	}}}
}

func dslRLSTestServer(t *testing.T) (*Server, context.Context, *metadata.Entity) {
	t.Helper()
	cat := &metadata.Entity{
		Name: "Товар", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Owner", Type: metadata.FieldTypeString},
		},
	}
	s, ctx := newSubmitTestServer(t, []*metadata.Entity{cat})
	if err := s.store.Upsert(ctx, cat.Name, uuid.New(), map[string]any{"Наименование": "Allowed", "Owner": "u"}, cat); err != nil {
		t.Fatalf("upsert allowed: %v", err)
	}
	if err := s.store.Upsert(ctx, cat.Name, uuid.New(), map[string]any{"Наименование": "Hidden", "Owner": "other"}, cat); err != nil {
		t.Fatalf("upsert hidden: %v", err)
	}
	return s, ctx, cat
}

func runDSLRowAccessFunc(t *testing.T, s *Server, ctx context.Context, src string) any {
	t.Helper()
	prog := mustParse(t, src)
	var result any
	vars := s.buildDSLVars(ctx, runtime.NewMovementsCollector("test", uuid.Nil))
	if err := s.interp.RunWithResult(prog.Procedures[0], nil, &result, vars); err != nil {
		t.Fatalf("run DSL: %v", err)
	}
	return result
}

func TestDSLQuery_RowAccessFiltersRows(t *testing.T) {
	s, _, _ := dslRLSTestServer(t)
	ctx := auth.ContextWithUser(context.Background(), dslRLSTestUser("u", "Товар", "read"))

	result := runDSLRowAccessFunc(t, s, ctx, `Функция Проверка() Экспорт
  З = Новый Запрос;
  З.Текст = "ВЫБРАТЬ Наименование ИЗ Справочник.Товар";
  Р = З.Выполнить();
  Возврат Р.Количество();
КонецФункции`)
	if result != float64(1) {
		t.Fatalf("DSL query row count = %v, want 1", result)
	}
}

func TestDSLCatalogFind_RowAccessHidesRows(t *testing.T) {
	s, _, _ := dslRLSTestServer(t)
	ctx := auth.ContextWithUser(context.Background(), dslRLSTestUser("u", "Товар", "read"))
	vars := s.buildDSLVars(ctx, runtime.NewMovementsCollector("test", uuid.Nil))
	catalogs := vars["Справочники"].(*interpreter.CatalogsRoot)
	proxy := catalogs.Get("Товар").(*interpreter.CatalogProxy)

	if got := proxy.CallMethod("найтипонаименованию", []any{"Hidden"}); got != nil {
		t.Fatalf("hidden row must not be found, got %T %+v", got, got)
	}
	if got := proxy.CallMethod("найтипонаименованию", []any{"Allowed"}); got == nil {
		t.Fatal("allowed row must be found")
	}
}

func TestTrustedOnWriteDSL_BypassesRowAccess(t *testing.T) {
	s, ctx, cat := dslRLSTestServer(t)
	doc := &metadata.Entity{
		Name: "Событие", Kind: metadata.KindDocument,
		Fields: []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
	}
	s.reg.Load(runtime.LoadOptions{
		Entities: []*metadata.Entity{cat, doc},
		Programs: map[string]*ast.Program{
			doc.Name: mustParse(t, `Процедура OnWrite() Экспорт
  З = Новый Запрос;
  З.Текст = "ВЫБРАТЬ Наименование ИЗ Справочник.Товар";
  Р = З.Выполнить();
  this.Количество = Р.Количество();
КонецПроцедуры`),
		},
	})
	userCtx := auth.ContextWithUser(ctx, dslRLSTestUser("u", "Товар", "read"))
	obj := runtime.NewObject(doc.Name, doc.Kind)
	mc := runtime.NewMovementsCollector(doc.Name, obj.ID)

	if errMsg, _ := s.runOnWriteCtx(userCtx, obj, mc); errMsg != "" {
		t.Fatalf("OnWrite error: %s", errMsg)
	}
	if got := obj.Get("Количество"); got != float64(2) {
		t.Fatalf("trusted OnWrite query count = %v, want 2", got)
	}
}
