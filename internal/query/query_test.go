package query_test

import (
	"strings"
	"testing"

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
