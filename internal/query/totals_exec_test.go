package query_test

import (
	"context"
	"math/rand"
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

// Остатки(&Момент) через итоги (этап 2) == расчёт на лету. Проверяем тождество
// на данных с движениями в разных месяцах, для набора моментов и с исключением
// регистратора по docID. Регистр с итогами (быстрый путь: месяцы до момента из
// итоги_* + хвост месяца из рег_*) и без итогов (on-the-fly) должны совпадать.
func TestBalancesTotals_MomentMatchesOnTheFly(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mkReg := func(name string, totals bool) *metadata.Register {
		return &metadata.Register{
			Name:       name,
			Dimensions: []metadata.Field{{Name: "Номенклатура", Type: metadata.FieldTypeString}},
			Resources:  []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
			Totals:     metadata.RegisterTotals{Enabled: totals},
		}
	}
	withT := mkReg("СИтогами", true)
	noT := mkReg("БезИтогов", false)
	regs := []*metadata.Register{withT, noT}
	if err := db.MigrateRegisters(ctx, regs); err != nil {
		t.Fatal(err)
	}

	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 12, 0, 0, 0, time.UTC) }
	rec1, rec2, rec3 := uuid.New(), uuid.New(), uuid.New()
	// Движения в трёх месяцах — в оба регистра одинаково.
	type wr struct {
		rec    uuid.UUID
		period time.Time
		rows   []map[string]any
	}
	writes := []wr{
		{rec1, d(2026, 1, 10), []map[string]any{{"ВидДвижения": "Приход", "Номенклатура": "Стол", "Количество": float64(5)}}},
		{rec2, d(2026, 2, 15), []map[string]any{
			{"ВидДвижения": "Приход", "Номенклатура": "Стол", "Количество": float64(3)},
			{"ВидДвижения": "Приход", "Номенклатура": "Стул", "Количество": float64(2)}}},
		{rec3, d(2026, 3, 20), []map[string]any{{"ВидДвижения": "Расход", "Номенклатура": "Стол", "Количество": float64(2)}}},
	}
	for _, w := range writes {
		for _, reg := range regs {
			p := w.period
			if err := db.WriteMovements(ctx, reg.Name, "Док", w.rec, w.rows, reg, &p); err != nil {
				t.Fatal(err)
			}
		}
	}

	balancesAt := func(reg *metadata.Register, mt *momentValue) map[string]float64 {
		r, err := query.Compile(
			"ВЫБРАТЬ Номенклатура, КоличествоОстаток ИЗ РегистрНакопления."+reg.Name+".Остатки(&МВ)",
			query.CompileOpts{Registers: regs, Params: map[string]any{"МВ": mt}, Dialect: storage.SQLiteDialect{}},
		)
		if err != nil {
			t.Fatalf("compile %s: %v", reg.Name, err)
		}
		if reg.Totals.Enabled && !strings.Contains(r.SQL, metadata.RegisterTotalsTableName(reg.Name)) {
			t.Errorf("Остатки(&Момент) с итогами должен читать %s, SQL: %s", metadata.RegisterTotalsTableName(reg.Name), r.SQL)
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

	moments := []*momentValue{
		{period: d(2026, 1, 1)},                      // до всех
		{period: d(2026, 1, 31)},                     // после января
		{period: d(2026, 2, 28)},                     // после февраля
		{period: d(2026, 3, 31)},                     // после марта
		{period: time.Now().UTC()},                   // текущий
		{period: d(2026, 2, 15), docID: rec2.String()}, // момент фев с исключением rec2
	}
	for i, mt := range moments {
		fast := balancesAt(withT, mt)
		slow := balancesAt(noT, mt)
		if len(fast) != len(slow) {
			t.Fatalf("момент %d: число строк итоги=%v на лету=%v", i, fast, slow)
		}
		for k, v := range slow {
			if fast[k] != v {
				t.Errorf("момент %d, %s: итоги=%v, на лету=%v", i, k, fast[k], v)
			}
		}
	}
}

// Рандомизированное тождество момента: случайные движения по нескольким месяцам
// и множество случайных моментов; итоги и on-the-fly совпадают везде.
func TestBalancesTotals_MomentRandomized(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mkReg := func(name string, totals bool) *metadata.Register {
		return &metadata.Register{
			Name:       name,
			Dimensions: []metadata.Field{{Name: "Товар", Type: metadata.FieldTypeString}},
			Resources:  []metadata.Field{{Name: "Кол", Type: metadata.FieldTypeNumber}},
			Totals:     metadata.RegisterTotals{Enabled: totals},
		}
	}
	withT, noT := mkReg("СИт", true), mkReg("БезИт", false)
	regs := []*metadata.Register{withT, noT}
	if err := db.MigrateRegisters(ctx, regs); err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(7))
	noms := []string{"A", "B", "C"}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 40; i++ {
		p := base.AddDate(0, 0, rng.Intn(180)) // ~6 месяцев
		vid := "Приход"
		if rng.Intn(2) == 0 {
			vid = "Расход"
		}
		row := []map[string]any{{"ВидДвижения": vid, "Товар": noms[rng.Intn(len(noms))], "Кол": float64(1 + rng.Intn(10))}}
		rec := uuid.New()
		for _, reg := range regs {
			pp := p
			if err := db.WriteMovements(ctx, reg.Name, "Д", rec, row, reg, &pp); err != nil {
				t.Fatal(err)
			}
		}
	}
	balancesAt := func(reg *metadata.Register, mt *momentValue) map[string]float64 {
		r, err := query.Compile("ВЫБРАТЬ Товар, КолОстаток ИЗ РегистрНакопления."+reg.Name+".Остатки(&МВ)",
			query.CompileOpts{Registers: regs, Params: map[string]any{"МВ": mt}, Dialect: storage.SQLiteDialect{}})
		if err != nil {
			t.Fatal(err)
		}
		rows, err := db.Query(ctx, r.SQL, r.Args...)
		if err != nil {
			t.Fatalf("exec: %v\nSQL: %s", err, r.SQL)
		}
		defer rows.Close()
		out := map[string]float64{}
		for rows.Next() {
			var n string
			var q float64
			if err := rows.Scan(&n, &q); err != nil {
				t.Fatal(err)
			}
			out[n] = q
		}
		return out
	}
	for i := 0; i < 30; i++ {
		mt := &momentValue{period: base.AddDate(0, 0, rng.Intn(200))}
		fast, slow := balancesAt(withT, mt), balancesAt(noT, mt)
		if len(fast) != len(slow) {
			t.Fatalf("момент %d (%s): итоги=%v на лету=%v", i, mt.period.Format("2006-01-02"), fast, slow)
		}
		for k, v := range slow {
			if fast[k] != v {
				t.Errorf("момент %d (%s) %s: итоги=%v на лету=%v", i, mt.period.Format("2006-01-02"), k, fast[k], v)
			}
		}
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
