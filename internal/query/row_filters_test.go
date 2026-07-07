package query_test

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
	"github.com/ivantit66/onebase/internal/storage"
)

func TestCompile_RowFiltersSimpleSourceAlias(t *testing.T) {
	cat := &metadata.Entity{
		Name: "Товар",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Owner", Type: metadata.FieldTypeString},
		},
	}
	res, err := query.Compile(`ВЫБРАТЬ Т.Наименование ИЗ Справочник.Товар КАК Т ГДЕ Т.Наименование <> &Пусто`, query.CompileOpts{
		Entities: []*metadata.Entity{cat},
		Params:   map[string]any{"Пусто": ""},
		Dialect:  storage.SQLiteDialect{},
		RowFilters: map[query.SourceRef]*storage.Predicate{
			{Kind: "catalog", Name: "Товар"}: {Field: "Owner", Op: "eq", Value: "u"},
		},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(res.SQL, "WHERE (т.owner = ?) AND") {
		t.Fatalf("row filter must be injected after WHERE with source alias, got:\n%s", res.SQL)
	}
	if len(res.Args) != 2 || res.Args[0] != "u" || res.Args[1] != "" {
		t.Fatalf("args = %#v, want row filter arg before query WHERE arg", res.Args)
	}
}

func TestCompile_RowFiltersInsertedBeforeOrder(t *testing.T) {
	cat := &metadata.Entity{
		Name:   "Товар",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Owner", Type: metadata.FieldTypeString}},
	}
	res, err := query.Compile(`ВЫБРАТЬ Ссылка ИЗ Справочник.Товар УПОРЯДОЧИТЬ ПО Ссылка`, query.CompileOpts{
		Entities: []*metadata.Entity{cat},
		Dialect:  storage.SQLiteDialect{},
		RowFilters: map[query.SourceRef]*storage.Predicate{
			{Kind: "catalog", Name: "Товар"}: {Field: "Owner", Op: "eq", Value: "u"},
		},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(res.SQL, "WHERE (товар.owner = ?) ORDER BY") {
		t.Fatalf("row filter must be inserted before ORDER BY, got:\n%s", res.SQL)
	}
}

func TestCompile_RowFiltersVirtualRegister(t *testing.T) {
	reg := &metadata.Register{
		Name: "ТоварноеДвижение",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: metadata.FieldTypeString},
			{Name: "Owner", Type: metadata.FieldTypeString},
		},
		Resources: []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
	}
	res, err := query.Compile(`ВЫБРАТЬ Номенклатура, КоличествоОстаток ИЗ РегистрНакопления.ТоварноеДвижение.Остатки()`, query.CompileOpts{
		Registers: []*metadata.Register{reg},
		Dialect:   storage.SQLiteDialect{},
		RowFilters: map[query.SourceRef]*storage.Predicate{
			{Kind: "register", Name: "ТоварноеДвижение"}: {Field: "Owner", Op: "eq", Value: "u"},
		},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(res.SQL, "FROM рег_товарноедвижение WHERE owner = ? GROUP BY") {
		t.Fatalf("row filter must be inside register virtual table, got:\n%s", res.SQL)
	}
}

func TestCompile_RowFiltersJoinedSourceScopedBeforeOn(t *testing.T) {
	doc := &metadata.Entity{Name: "Заказ", Kind: metadata.KindDocument}
	cat := &metadata.Entity{
		Name: "Клиент",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Owner", Type: metadata.FieldTypeString},
		},
	}
	res, err := query.Compile(`ВЫБРАТЬ з.Ссылка, к.Наименование ИЗ Документ.Заказ КАК з ЛЕВОЕ СОЕДИНЕНИЕ Справочник.Клиент КАК к ПО к.Ссылка = з.Ссылка`, query.CompileOpts{
		Entities: []*metadata.Entity{doc, cat},
		Dialect:  storage.SQLiteDialect{},
		RowFilters: map[query.SourceRef]*storage.Predicate{
			{Kind: "catalog", Name: "Клиент"}: {Field: "Owner", Op: "eq", Value: "u"},
		},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(res.SQL, "LEFT JOIN (SELECT * FROM клиент WHERE owner = ?) AS к ON") {
		t.Fatalf("joined row filter must be scoped inside joined source, got:\n%s", res.SQL)
	}
	if strings.Contains(res.SQL, "WHERE (к.owner = ?)") {
		t.Fatalf("joined row filter must not turn LEFT JOIN into an outer WHERE filter:\n%s", res.SQL)
	}
	if len(res.Args) != 1 || res.Args[0] != "u" {
		t.Fatalf("args = %#v, want one joined row filter arg", res.Args)
	}
}

// TestCompile_RowFiltersSubqueryInFromFailClosed: у ограниченного источника,
// оказавшегося главной таблицей внутри подзапроса ИЗ, отложенный фильтр нельзя
// корректно поместить в outer WHERE (раньше выходил битый SQL). Теперь — явный
// fail-closed отказ, а не тихая утечка/поломка.
func TestCompile_RowFiltersSubqueryInFromFailClosed(t *testing.T) {
	cat := &metadata.Entity{
		Name:   "Товар",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Owner", Type: metadata.FieldTypeString}},
	}
	_, err := query.Compile(`ВЫБРАТЬ * ИЗ (ВЫБРАТЬ Ссылка ИЗ Справочник.Товар) КАК П`, query.CompileOpts{
		Entities: []*metadata.Entity{cat},
		Dialect:  storage.SQLiteDialect{},
		RowFilters: map[query.SourceRef]*storage.Predicate{
			{Kind: "catalog", Name: "Товар"}: {Field: "Owner", Op: "eq", Value: "u"},
		},
	})
	if err == nil {
		t.Fatal("restricted source inside a FROM subquery must fail closed, got nil error")
	}
	if !strings.Contains(err.Error(), "подзапрос") {
		t.Fatalf("error must explain the FROM-subquery limitation, got: %v", err)
	}
}

// TestCompile_SubqueryInFromOpenDeployment: без активных строковых политик
// подзапрос в ИЗ по-прежнему компилируется (отказ касается только ограниченных
// источников — pred!=nil).
func TestCompile_SubqueryInFromOpenDeployment(t *testing.T) {
	cat := &metadata.Entity{
		Name:   "Товар",
		Kind:   metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	res, err := query.Compile(`ВЫБРАТЬ * ИЗ (ВЫБРАТЬ Ссылка ИЗ Справочник.Товар) КАК П`, query.CompileOpts{
		Entities: []*metadata.Entity{cat},
		Dialect:  storage.SQLiteDialect{},
	})
	if err != nil {
		t.Fatalf("open deployment must still compile FROM subqueries: %v", err)
	}
	if !strings.Contains(res.SQL, "SELECT id FROM товар") {
		t.Fatalf("FROM subquery must survive without row filters, got:\n%s", res.SQL)
	}
}
