package query_test

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
)

func TestCompile_BalancesQuery(t *testing.T) {
	src := `ВЫБРАТЬ
  Номенклатура,
  СУММА(Количество) КАК Количество
ИЗ РегистрНакопления.ТоварноеДвижение
СГРУППИРОВАТЬ ПО Номенклатура
УПОРЯДОЧИТЬ ПО Номенклатура`

	r, err := query.Compile(src, query.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL
	if !strings.Contains(sql, "SELECT") {
		t.Errorf("expected SELECT, got: %s", sql)
	}
	if !strings.Contains(sql, "SUM(количество)") {
		t.Errorf("expected SUM(количество), got: %s", sql)
	}
	if !strings.Contains(sql, "рег_товарноедвижение") {
		t.Errorf("expected рег_товарноедвижение, got: %s", sql)
	}
	if !strings.Contains(sql, "GROUP BY") {
		t.Errorf("expected GROUP BY, got: %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY") {
		t.Errorf("expected ORDER BY, got: %s", sql)
	}
	if len(r.Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(r.Args))
	}
}

func TestCompile_WithParam(t *testing.T) {
	src := `ВЫБРАТЬ Номенклатура ИЗ РегистрНакопления.ТоварноеДвижение ГДЕ вид_движения = &ВидДвижения`

	r, err := query.Compile(src, query.CompileOpts{Params: map[string]any{"ВидДвижения": "Приход"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "$1") {
		t.Errorf("expected $1 placeholder, got: %s", r.SQL)
	}
	if len(r.Args) != 1 || r.Args[0] != "Приход" {
		t.Errorf("expected args=[Приход], got %v", r.Args)
	}
}

func TestCompile_WithUUIDParam(t *testing.T) {
	src := `ВЫБРАТЬ Номенклатура ИЗ РегистрНакопления.ТоварноеДвижение ГДЕ Номенклатура = &Ном`

	r, err := query.Compile(src, query.CompileOpts{Params: map[string]any{"Ном": "4e582af9-cd26-4af0-a244-d282e02a5603"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "::uuid") {
		t.Errorf("expected ::uuid cast for UUID param, got: %s", r.SQL)
	}
	if len(r.Args) != 1 || r.Args[0] != "4e582af9-cd26-4af0-a244-d282e02a5603" {
		t.Errorf("expected UUID arg, got %v", r.Args)
	}
}

func TestCompile_WithNonUUIDStringParam(t *testing.T) {
	src := `ВЫБРАТЬ Номенклатура ИЗ РегистрНакопления.ТоварноеДвижение ГДЕ вид_движения = &Вид`

	r, err := query.Compile(src, query.CompileOpts{Params: map[string]any{"Вид": "Приход"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "::text") {
		t.Errorf("expected ::text cast for regular string param, got: %s", r.SQL)
	}
}

func TestCompile_StringLiteral(t *testing.T) {
	src := `ВЫБРАТЬ Номенклатура ИЗ РегистрНакопления.ТоварноеДвижение ГДЕ вид_движения = "Приход"`

	r, err := query.Compile(src, query.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "'Приход'") {
		t.Errorf("expected single-quoted string, got: %s", r.SQL)
	}
}

func TestCompile_InnerJoin(t *testing.T) {
	src := `ВЫБРАТЬ
  Прод.Номер,
  Клиент.Наименование,
  Прод.Сумма
ИЗ Документ.Реализация КАК Прод
  ВНУТРЕННЕЕ СОЕДИНЕНИЕ Справочник.Клиент КАК Клиент
  ПО Прод.Покупатель = Клиент.Ссылка`

	r, err := query.Compile(src, query.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL
	if !strings.Contains(sql, "INNER JOIN") {
		t.Errorf("expected INNER JOIN, got: %s", sql)
	}
	if !strings.Contains(sql, "реализация") {
		t.Errorf("expected реализация table, got: %s", sql)
	}
	if !strings.Contains(sql, "клиент") {
		t.Errorf("expected клиент table, got: %s", sql)
	}
	// .Ссылка should map to .id
	if !strings.Contains(sql, "клиент.id") {
		t.Errorf("expected клиент.id (Ссылка→id), got: %s", sql)
	}
	if !strings.Contains(sql, "ON") {
		t.Errorf("expected ON clause, got: %s", sql)
	}
}

func TestCompile_LeftJoin_WithGroupBy(t *testing.T) {
	src := `ВЫБРАТЬ
  Н.Наименование,
  СУММА(Д.Количество) КАК Итог
ИЗ Справочник.Номенклатура КАК Н
  ЛЕВОЕ СОЕДИНЕНИЕ РегистрНакопления.ОстаткиТоваров КАК Д
  ПО Н.Ссылка = Д.Номенклатура
СГРУППИРОВАТЬ ПО Н.Наименование
УПОРЯДОЧИТЬ ПО Н.Наименование`

	r, err := query.Compile(src, query.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL
	if !strings.Contains(sql, "LEFT JOIN") {
		t.Errorf("expected LEFT JOIN, got: %s", sql)
	}
	if !strings.Contains(sql, "GROUP BY") {
		t.Errorf("expected GROUP BY, got: %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY") {
		t.Errorf("expected ORDER BY, got: %s", sql)
	}
	// .Ссылка → .id
	if !strings.Contains(sql, "н.id") {
		t.Errorf("expected н.id (Ссылка→id), got: %s", sql)
	}
	// ON keyword (not BY)
	if !strings.Contains(sql, "ON") {
		t.Errorf("expected ON clause, got: %s", sql)
	}
}

func TestCompile_EnglishJoin(t *testing.T) {
	src := `SELECT P.Number, C.Name
FROM Document.Sale AS P
  LEFT JOIN Catalog.Client AS C
  ON P.Client = C.Reference`

	r, err := query.Compile(src, query.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL
	if !strings.Contains(sql, "LEFT JOIN") {
		t.Errorf("expected LEFT JOIN, got: %s", sql)
	}
	if !strings.Contains(sql, "c.id") {
		t.Errorf("expected c.id (Reference→id), got: %s", sql)
	}
}

func TestCompile_RightJoin(t *testing.T) {
	src := `ВЫБРАТЬ К.Наименование
ИЗ Документ.Заказ КАК З
  ПРАВОЕ СОЕДИНЕНИЕ Справочник.Клиент КАК К
  ПО З.Клиент = К.Ссылка`

	r, err := query.Compile(src, query.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "RIGHT JOIN") {
		t.Errorf("expected RIGHT JOIN, got: %s", r.SQL)
	}
}

func TestCompile_Ssylka_InSelect(t *testing.T) {
	src := `ВЫБРАТЬ Н.Ссылка, Н.Наименование ИЗ Справочник.Номенклатура КАК Н`

	r, err := query.Compile(src, query.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	// Н.Ссылка → н.id
	if !strings.Contains(r.SQL, "н.id") {
		t.Errorf("expected н.id, got: %s", r.SQL)
	}
}

func TestCompile_RefDim_AutoJoin(t *testing.T) {
	// When a register has a reference-type dimension, the query compiler should:
	// • SELECT:   Номенклатура → ref_номенклатура.наименование AS номенклатура
	// • FROM:     inject LEFT JOIN номенклатура ref_номенклатура ON ...
	// • WHERE:    Номенклатура → номенклатура_id
	// • GROUP BY: Номенклатура → ref_номенклатура.наименование
	src := `ВЫБРАТЬ
  Номенклатура,
  СУММА(Выручка) КАК Выручка
ИЗ РегистрНакопления.ВаловаяПрибыль
ГДЕ (&Номенклатура ЕСТЬ ПУСТО ИЛИ Номенклатура = &Номенклатура)
СГРУППИРОВАТЬ ПО Номенклатура`

	reg := &metadata.Register{
		Name: "ВаловаяПрибыль",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", RefEntity: "Номенклатура"},
		},
		Resources: []metadata.Field{
			{Name: "Выручка"},
		},
	}

	r, err := query.Compile(src, query.CompileOpts{
		Registers: []*metadata.Register{reg},
	})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL

	// SELECT must reference наименование from the join alias, not the raw dim name
	if !strings.Contains(sql, "ref_номенклатура.наименование") {
		t.Errorf("expected ref_номенклатура.наименование in SELECT, got: %s", sql)
	}
	// Auto-JOIN must be present
	if !strings.Contains(sql, "LEFT JOIN номенклатура ref_номенклатура") {
		t.Errorf("expected LEFT JOIN, got: %s", sql)
	}
	// WHERE must use the _id column
	if !strings.Contains(sql, "номенклатура_id") {
		t.Errorf("expected номенклатура_id in WHERE, got: %s", sql)
	}
	// GROUP BY must use the join expression (not _id)
	if !strings.Contains(sql, "GROUP BY ref_номенклатура.наименование") {
		t.Errorf("expected GROUP BY ref_номенклатура.наименование, got: %s", sql)
	}
}

// TestCompile_StringDim_NoIdSuffix ensures that a string-type dimension
// in one register is not incorrectly mapped to _id when another register
// in opts has a reference-type dimension with the same name.
func TestCompile_StringDim_NoIdSuffix(t *testing.T) {
	src := `ВЫБРАТЬ
  Номенклатура,
  Склад,
  СУММА(Количество) КАК Количество
ИЗ РегистрНакопления.ОстаткиТоваров
СГРУППИРОВАТЬ ПО Номенклатура, Склад`

	// ОстаткиТоваров has Номенклатура as plain string
	regStock := &metadata.Register{
		Name: "ОстаткиТоваров",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: "string"},
			{Name: "Склад", Type: "string"},
		},
		Resources: []metadata.Field{{Name: "Количество"}},
	}
	// ВаловаяПрибыль has Номенклатура as reference — should NOT pollute colMap
	regProfit := &metadata.Register{
		Name: "ВаловаяПрибыль",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: "reference:Номенклатура", RefEntity: "Номенклатура"},
		},
		Resources: []metadata.Field{{Name: "Выручка"}},
	}

	r, err := query.Compile(src, query.CompileOpts{
		Registers: []*metadata.Register{regStock, regProfit},
	})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL
	if strings.Contains(sql, "номенклатура_id") {
		t.Errorf("string-type dim must NOT become _id; got: %s", sql)
	}
	if !strings.Contains(sql, "номенклатура") {
		t.Errorf("expected plain 'номенклатура' column in SQL, got: %s", sql)
	}
}

// TestCompile_BareCatalogInFrom ensures that a bare catalog name in FROM clause
// is not replaced by colMap entry from a reference dimension in another register.
// Regression: "ИЗ Номенклатура" was compiled to "FROM номенклатура_id" because
// colMap fallback mapped it from a register with reference:Номенклатура.
func TestCompile_BareCatalogInFrom(t *testing.T) {
	src := "ВЫБРАТЬ Наименование, ЦенаПродажи ИЗ Номенклатура"

	regProfit := &metadata.Register{
		Name: "ВаловаяПрибыль",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: "reference:Номенклатура", RefEntity: "Номенклатура"},
		},
		Resources: []metadata.Field{{Name: "Выручка"}},
	}

	r, err := query.Compile(src, query.CompileOpts{
		Registers: []*metadata.Register{regProfit},
	})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL
	if !strings.Contains(sql, "FROM номенклатура") {
		t.Errorf("expected FROM номенклатура, got: %s", sql)
	}
	if strings.Contains(sql, "FROM номенклатура_id") {
		t.Errorf("bare catalog name must NOT be replaced by _id column; got: %s", sql)
	}
}

// TestCompile_VT_RefDim_AutoJoin ensures that VT queries (Остатки, etc.) also
// get auto-JOINs for reference dimensions so results show names, not UUIDs.
func TestCompile_VT_RefDim_AutoJoin(t *testing.T) {
	src := "ВЫБРАТЬ Номенклатура КАК Ном, КоличествоОстаток КАК Количество " +
		"ИЗ РегистрНакопления.ПартииТоваров.Остатки() " +
		"ГДЕ (&Номенклатура ЕСТЬ ПУСТО ИЛИ Номенклатура = &Номенклатура) " +
		"УПОРЯДОЧИТЬ ПО Ном"

	reg := &metadata.Register{
		Name: "ПартииТоваров",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", RefEntity: "Номенклатура"},
		},
		Resources: []metadata.Field{{Name: "Количество"}},
	}

	r, err := query.Compile(src, query.CompileOpts{
		Registers: []*metadata.Register{reg},
		Params:    map[string]any{"Номенклатура": nil},
	})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL

	// SELECT: must resolve ref dim via JOIN, not double AS
	if !strings.Contains(sql, "ref_номенклатура.наименование AS ном") {
		t.Errorf("expected ref_номенклатура.наименование AS ном, got: %s", sql)
	}
	// LEFT JOIN must be present in outer query
	if !strings.Contains(sql, "LEFT JOIN номенклатура ref_номенклатура") {
		t.Errorf("expected LEFT JOIN for VT outer query, got: %s", sql)
	}
	// WHERE: outer query must use logical name (aliased from VT subquery)
	if !strings.Contains(sql, "номенклатура = NULL") {
		t.Errorf("expected logical name in outer WHERE, got: %s", sql)
	}
	// ORDER BY: alias is fine
	if !strings.Contains(sql, "ORDER BY ном") {
		t.Errorf("expected ORDER BY ном (alias), got: %s", sql)
	}
}

// TestCompile_VT_WithUserAlias verifies that КАК after a VT subquery is consumed
// and the user-provided alias is used in the auto-JOIN ON clause.
func TestCompile_VT_WithUserAlias(t *testing.T) {
	src := "ВЫБРАТЬ Номенклатура ИЗ РегистрНакопления.ПартииТоваров.Остатки() КАК Пар"

	reg := &metadata.Register{
		Name: "ПартииТоваров",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", RefEntity: "Номенклатура"},
		},
		Resources: []metadata.Field{{Name: "Количество"}},
	}

	r, err := query.Compile(src, query.CompileOpts{
		Registers: []*metadata.Register{reg},
	})
	if err != nil {
		t.Fatal(err)
	}
	sql := r.SQL

	if !strings.Contains(sql, "AS пар") {
		t.Errorf("expected user alias 'пар', got: %s", sql)
	}
	if strings.Contains(sql, "AS остатки_партиитоваров") {
		t.Errorf("default alias must not appear when user provides КАК, got: %s", sql)
	}
	if !strings.Contains(sql, "пар.номенклатура") {
		t.Errorf("JOIN ON must use user alias, got: %s", sql)
	}
}
