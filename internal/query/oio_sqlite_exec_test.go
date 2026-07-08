package query_test

import (
	"context"
	"math"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
	"github.com/ivantit66/onebase/internal/storage"
)

// ОстаткиИОбороты(&Начало, &Конец) с датами-ПАРАМЕТРАМИ должна исполняться на
// SQLite и давать корректные значения. Раньше границы транслировались один раз,
// а анонимный '?' вставлялся многократно → «missing argument». Регрессионный тест
// сверяет все колонки с эталоном, вычисленным в Go напрямую из движений.
func TestBalancesAndTurnovers_ParamDatesSQLite(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reg := &metadata.Register{
		Name:       "ОИ",
		Dimensions: []metadata.Field{{Name: "Товар", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Кол", Type: metadata.FieldTypeNumber}},
	}
	if err := db.MigrateRegisters(ctx, []*metadata.Register{reg}); err != nil {
		t.Fatal(err)
	}
	type mvRec struct {
		nom, vid string
		period   time.Time
		qty      float64
	}
	var all []mvRec
	rng := rand.New(rand.NewSource(3))
	noms := []string{"A", "B", "C"}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 40; i++ {
		p := base.AddDate(0, 0, rng.Intn(180))
		vid := "Приход"
		if rng.Intn(2) == 0 {
			vid = "Расход"
		}
		nom := noms[rng.Intn(len(noms))]
		qty := float64(1 + rng.Intn(10))
		all = append(all, mvRec{nom, vid, p, qty})
		pp := p
		if err := db.WriteMovements(ctx, reg.Name, "Д", uuid.New(),
			[]map[string]any{{"ВидДвижения": vid, "Товар": nom, "Кол": qty}}, reg, &pp); err != nil {
			t.Fatal(err)
		}
	}
	golden := func(start, end time.Time) map[string][4]float64 {
		out := map[string][4]float64{}
		for _, m := range all {
			signed := m.qty
			if m.vid == "Расход" {
				signed = -m.qty
			}
			v := out[m.nom]
			if m.period.Before(start) {
				v[0] += signed
			}
			if !m.period.Before(start) && !m.period.After(end) {
				if m.vid == "Приход" {
					v[1] += m.qty
				} else {
					v[2] += m.qty
				}
			}
			if !m.period.After(end) {
				v[3] += signed
			}
			out[m.nom] = v
		}
		for nom := range out {
			has := false
			for _, m := range all {
				if m.nom == nom && !m.period.After(end) {
					has = true
					break
				}
			}
			if !has {
				delete(out, nom)
			}
		}
		return out
	}
	for i := 0; i < 25; i++ {
		s := base.AddDate(0, 0, rng.Intn(150))
		e := s.AddDate(0, 0, 1+rng.Intn(90))
		r, err := query.Compile(
			"ВЫБРАТЬ Товар, Колначальный, Колприход, Колрасход, Колконечный ИЗ РегистрНакопления.ОИ.ОстаткиИОбороты(&Н, &К)",
			query.CompileOpts{Registers: []*metadata.Register{reg}, Params: map[string]any{"Н": s, "К": e}, Dialect: storage.SQLiteDialect{}})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		rows, err := db.Query(ctx, r.SQL, r.Args...)
		if err != nil {
			t.Fatalf("exec (баг плейсхолдеров?): %v\nSQL: %s\nArgs: %v", err, r.SQL, r.Args)
		}
		got := map[string][4]float64{}
		for rows.Next() {
			var n string
			var a, b, c, dd float64
			if err := rows.Scan(&n, &a, &b, &c, &dd); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			got[n] = [4]float64{a, b, c, dd}
		}
		rows.Close()
		want := golden(s, e)
		if len(got) != len(want) {
			t.Fatalf("период %d [%s..%s]: строк got=%d want=%d\ngot=%v\nwant=%v",
				i, s.Format("01-02"), e.Format("01-02"), len(got), len(want), got, want)
		}
		for k, w := range want {
			g := got[k]
			for j := 0; j < 4; j++ {
				if math.Abs(g[j]-w[j]) > 1e-9 {
					t.Errorf("период %d [%s..%s] %s поле %d: got=%v want=%v",
						i, s.Format("01-02"), e.Format("01-02"), k, j, g[j], w[j])
				}
			}
		}
	}
}
