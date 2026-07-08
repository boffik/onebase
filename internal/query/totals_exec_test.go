package query_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
	"github.com/ivantit66/onebase/internal/storage"
)

// План 80: Остатки() без момента для регистра с включёнными итогами читает
// таблицу итоги_* (быстрый путь), а не суммирует движения. Результат совпадает
// с обычным путём (регистр без итогов) на тех же данных.
func TestBalancesTotals_FastPathExecutes(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	withTotals := &metadata.Register{
		Name:       "ОстаткиТоваров",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
		Totals:     metadata.RegisterTotals{Enabled: true},
	}
	noTotals := &metadata.Register{
		Name:       "ОстаткиБезИтогов",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
	}
	regs := []*metadata.Register{withTotals, noTotals}
	if err := db.MigrateRegisters(ctx, regs); err != nil {
		t.Fatal(err)
	}

	p := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	// Стол: +5, +3 = 8; Стул: +2 = 2 — в оба регистра.
	for _, reg := range regs {
		if err := db.WriteMovements(ctx, reg.Name, "Пост", uuid.New(),
			[]map[string]any{
				{"ВидДвижения": "Приход", "Номенклатура": "Стол", "Количество": float64(5)},
				{"ВидДвижения": "Приход", "Номенклатура": "Стул", "Количество": float64(2)},
			}, reg, &p); err != nil {
			t.Fatal(err)
		}
		if err := db.WriteMovements(ctx, reg.Name, "Пост", uuid.New(),
			[]map[string]any{{"ВидДвижения": "Приход", "Номенклатура": "Стол", "Количество": float64(3)}},
			reg, &p); err != nil {
			t.Fatal(err)
		}
	}

	balances := func(reg *metadata.Register) map[string]float64 {
		r, err := query.Compile(
			"ВЫБРАТЬ Номенклатура, КоличествоОстаток ИЗ РегистрНакопления."+reg.Name+".Остатки()",
			query.CompileOpts{Registers: regs, Dialect: storage.SQLiteDialect{}},
		)
		if err != nil {
			t.Fatalf("compile %s: %v", reg.Name, err)
		}
		totalsTable := metadata.RegisterTotalsTableName(reg.Name)
		if reg.Totals.Enabled {
			if !strings.Contains(r.SQL, totalsTable) {
				t.Errorf("Остатки() для регистра с итогами должен читать %s, SQL: %s", totalsTable, r.SQL)
			}
		} else if strings.Contains(r.SQL, totalsTable) {
			t.Errorf("регистр без итогов не должен читать %s, SQL: %s", totalsTable, r.SQL)
		}
		rows, err := db.Query(ctx, r.SQL, r.Args...)
		if err != nil {
			t.Fatalf("exec %s: %v\nSQL: %s", reg.Name, err, r.SQL)
		}
		defer rows.Close()
		out := map[string]float64{}
		for rows.Next() {
			var name string
			var qty float64
			if err := rows.Scan(&name, &qty); err != nil {
				t.Fatal(err)
			}
			out[name] = qty
		}
		return out
	}

	fast := balances(withTotals)
	slow := balances(noTotals)
	if fast["Стол"] != 8 || fast["Стул"] != 2 {
		t.Errorf("быстрый путь: Стол=%v Стул=%v, ожидалось 8 и 2", fast["Стол"], fast["Стул"])
	}
	if fast["Стол"] != slow["Стол"] || fast["Стул"] != slow["Стул"] {
		t.Errorf("быстрый путь разошёлся с обычным: %v vs %v", fast, slow)
	}
}

// Остатки(&Момент) не использует быстрый путь даже при включённых итогах:
// итоги независимы от времени и не отвечают на «остаток на дату» (этап 2).
func TestBalancesTotals_MomentFallsBack(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	reg := &metadata.Register{
		Name:       "ОстаткиТоваров",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
		Totals:     metadata.RegisterTotals{Enabled: true},
	}
	if err := db.MigrateRegisters(ctx, []*metadata.Register{reg}); err != nil {
		t.Fatal(err)
	}

	mt := &momentValue{period: time.Now(), docID: uuid.New().String()}
	r, err := query.Compile(
		"ВЫБРАТЬ Номенклатура, КоличествоОстаток ИЗ РегистрНакопления.ОстаткиТоваров.Остатки(&МВ)",
		query.CompileOpts{Registers: []*metadata.Register{reg}, Params: map[string]any{"МВ": mt}, Dialect: storage.SQLiteDialect{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.SQL, metadata.RegisterTotalsTableName(reg.Name)) {
		t.Errorf("Остатки(&Момент) не должен использовать итоги (этап 1), SQL: %s", r.SQL)
	}
	if !strings.Contains(r.SQL, metadata.RegisterTableName(reg.Name)) {
		t.Errorf("Остатки(&Момент) должен читать движения %s, SQL: %s", metadata.RegisterTableName(reg.Name), r.SQL)
	}
}

// Регистр с атрибутами не использует итоги (их таблица атрибуты не хранит):
// Остатки() идёт обычным путём даже при totals.enabled — этап 1.
func TestBalancesTotals_AttributesFallBack(t *testing.T) {
	reg := &metadata.Register{
		Name:       "ОстаткиСАтрибутом",
		Dimensions: []metadata.Field{{Name: "Товар", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Кол", Type: metadata.FieldTypeNumber}},
		Attributes: []metadata.Field{{Name: "Комментарий", Type: metadata.FieldTypeString}},
		Totals:     metadata.RegisterTotals{Enabled: true},
	}
	r, err := query.Compile(
		"ВЫБРАТЬ Товар, КолОстаток ИЗ РегистрНакопления.ОстаткиСАтрибутом.Остатки()",
		query.CompileOpts{Registers: []*metadata.Register{reg}, Dialect: storage.SQLiteDialect{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.SQL, metadata.RegisterTotalsTableName(reg.Name)) {
		t.Errorf("регистр с атрибутами не должен читать итоги, SQL: %s", r.SQL)
	}
}
