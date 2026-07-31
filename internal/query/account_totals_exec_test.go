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

// Быстрый путь остатков бухрегистра через итоги (план 80) даёт те же сальдо, что и
// расчёт на лету: один и тот же запрос компилируется с включёнными и выключенными
// итогами, исполняется на одной БД, результаты сравниваются.
func TestAccountBalances_TotalsFastPathMatchesOnTheFly(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ar := &metadata.AccountRegister{
		Name:      "БухПаритет",
		Accounts:  "Основной",
		Resources: []metadata.Field{{Name: "Сумма", Type: "number"}},
		Totals:    metadata.RegisterTotals{Enabled: true},
	}
	if err := db.MigrateAccountRegisters(ctx, []*metadata.AccountRegister{ar}); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureAccountsTable(ctx); err != nil {
		t.Fatal(err)
	}
	chart := &metadata.ChartOfAccounts{
		Name: "Основной",
		Accounts: []metadata.Account{
			{Code: "41", Kind: "active"},
			{Code: "60", Kind: "passive"},
			{Code: "51", Kind: "active"},
			{Code: "62", Kind: "passive"},
			{Code: "90", Kind: "passive"},
		},
	}
	if err := db.SyncAccounts(ctx, []*metadata.ChartOfAccounts{chart}); err != nil {
		t.Fatal(err)
	}

	june := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC)
	if err := db.WriteAccountMovements(ctx, ar.Name, "Док", uuid.New(), []map[string]any{
		{"счётдт": "41", "счёткт": "60", "сумма": float64(1000)},
		{"счётдт": "51", "счёткт": "62", "сумма": float64(700)},
	}, ar, &june); err != nil {
		t.Fatal(err)
	}
	if err := db.WriteAccountMovements(ctx, ar.Name, "Док", uuid.New(), []map[string]any{
		{"счётдт": "41", "счёткт": "60", "сумма": float64(300)},
		{"счётдт": "62", "счёткт": "90", "сумма": float64(700)},
	}, ar, &july); err != nil {
		t.Fatal(err)
	}

	const src = "ВЫБРАТЬ Счёт, СуммаОстаток ИЗ РегистрБухгалтерии.БухПаритет.Остатки()"

	run := func(totals bool) map[string]float64 {
		reg := *ar
		reg.Totals = metadata.RegisterTotals{Enabled: totals}
		res, err := query.Compile(src, query.CompileOpts{
			Dialect:     db.Dialect(),
			AccountRegs: []*metadata.AccountRegister{&reg},
		})
		if err != nil {
			t.Fatalf("compile (totals=%v): %v", totals, err)
		}
		if usesTotals := strings.Contains(res.SQL, "итоги_акк_"); usesTotals != totals {
			t.Fatalf("totals=%v, но признак итогов в SQL=%v: %s", totals, usesTotals, res.SQL)
		}
		rows, err := db.Query(ctx, res.SQL, res.Args...)
		if err != nil {
			t.Fatalf("exec (totals=%v): %v\nSQL: %s", totals, err, res.SQL)
		}
		defer rows.Close()
		out := map[string]float64{}
		for rows.Next() {
			var acc string
			var bal float64
			if err := rows.Scan(&acc, &bal); err != nil {
				t.Fatal(err)
			}
			out[acc] = bal
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}

	fast := run(true)
	slow := run(false)

	if len(fast) == 0 {
		t.Fatal("быстрый путь не вернул строк")
	}
	if len(fast) != len(slow) {
		t.Fatalf("число счетов: итоги=%d, на лету=%d\n%v\n%v", len(fast), len(slow), fast, slow)
	}
	for acc, bal := range slow {
		if fast[acc] != bal {
			t.Errorf("счёт %s: сальдо итоги=%v, на лету=%v", acc, fast[acc], bal)
		}
	}
	// смысловая проверка: 41 дебетовое +1300, 60 кредитовое -1300, 90 -700, 62 = 0.
	if fast["41"] != 1300 {
		t.Errorf("41: ожидалось 1300, получили %v", fast["41"])
	}
	if fast["60"] != -1300 {
		t.Errorf("60: ожидалось -1300, получили %v", fast["60"])
	}
	if fast["90"] != -700 {
		t.Errorf("90: ожидалось -700, получили %v", fast["90"])
	}
}

// Остатки(&НаДату) бухрегистра через итоги == расчёт на лету на разных датах:
// проверяет разворот «месяцы до момента из итогов + хвост проводок месяца момента».
func TestAccountBalances_TotalsAtMomentMatchesOnTheFly(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ar := &metadata.AccountRegister{
		Name:      "БухНаМомент",
		Accounts:  "Основной",
		Resources: []metadata.Field{{Name: "Сумма", Type: "number"}},
		Totals:    metadata.RegisterTotals{Enabled: true},
	}
	if err := db.MigrateAccountRegisters(ctx, []*metadata.AccountRegister{ar}); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureAccountsTable(ctx); err != nil {
		t.Fatal(err)
	}
	chart := &metadata.ChartOfAccounts{Name: "Основной", Accounts: []metadata.Account{
		{Code: "41", Kind: "active"}, {Code: "60", Kind: "passive"},
		{Code: "51", Kind: "active"}, {Code: "62", Kind: "passive"}, {Code: "90", Kind: "passive"},
	}}
	if err := db.SyncAccounts(ctx, []*metadata.ChartOfAccounts{chart}); err != nil {
		t.Fatal(err)
	}
	june := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC)
	if err := db.WriteAccountMovements(ctx, ar.Name, "Док", uuid.New(), []map[string]any{
		{"счётдт": "41", "счёткт": "60", "сумма": float64(1000)},
		{"счётдт": "51", "счёткт": "62", "сумма": float64(700)},
	}, ar, &june); err != nil {
		t.Fatal(err)
	}
	if err := db.WriteAccountMovements(ctx, ar.Name, "Док", uuid.New(), []map[string]any{
		{"счётдт": "41", "счёткт": "60", "сумма": float64(300)},
		{"счётдт": "62", "счёткт": "90", "сумма": float64(700)},
	}, ar, &july); err != nil {
		t.Fatal(err)
	}

	const src = "ВЫБРАТЬ Счёт, СуммаОстаток ИЗ РегистрБухгалтерии.БухНаМомент.Остатки(&НаДату)"
	run := func(totals bool, onDate time.Time) map[string]float64 {
		reg := *ar
		reg.Totals = metadata.RegisterTotals{Enabled: totals}
		res, err := query.Compile(src, query.CompileOpts{
			Dialect:     db.Dialect(),
			AccountRegs: []*metadata.AccountRegister{&reg},
			Params:      map[string]any{"НаДату": onDate},
		})
		if err != nil {
			t.Fatalf("compile totals=%v: %v", totals, err)
		}
		if uses := strings.Contains(res.SQL, "итоги_акк_"); uses != totals {
			t.Fatalf("totals=%v, признак итогов в SQL=%v: %s", totals, uses, res.SQL)
		}
		rows, err := db.Query(ctx, res.SQL, res.Args...)
		if err != nil {
			t.Fatalf("exec totals=%v: %v\nSQL: %s\nArgs: %v", totals, err, res.SQL, res.Args)
		}
		defer rows.Close()
		out := map[string]float64{}
		for rows.Next() {
			var acc string
			var bal float64
			if err := rows.Scan(&acc, &bal); err != nil {
				t.Fatal(err)
			}
			out[acc] = bal
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}

	cases := []struct {
		name string
		date time.Time
	}{
		{"до всех движений", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"конец июня (только июнь, хвост без итогов)", time.Date(2026, 6, 30, 23, 0, 0, 0, time.UTC)},
		{"середина июля (итоги июня + хвост июля)", time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fast := run(true, c.date)
			slow := run(false, c.date)
			if len(fast) != len(slow) {
				t.Fatalf("число счетов: итоги=%d, на лету=%d\n%v\n%v", len(fast), len(slow), fast, slow)
			}
			for acc, bal := range slow {
				if fast[acc] != bal {
					t.Errorf("счёт %s: сальдо итоги=%v, на лету=%v", acc, fast[acc], bal)
				}
			}
		})
	}
}

// Обороты бухрегистра через итоги == расчёт на лету: полный период, месяц целиком,
// и диапазон, охватывающий полный средний месяц из итогов + хвосты по краям.
func TestAccountTurnovers_TotalsFastPathMatchesOnTheFly(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ar := &metadata.AccountRegister{
		Name:      "БухОбороты",
		Accounts:  "Основной",
		Resources: []metadata.Field{{Name: "Сумма", Type: "number"}},
		Totals:    metadata.RegisterTotals{Enabled: true},
	}
	if err := db.MigrateAccountRegisters(ctx, []*metadata.AccountRegister{ar}); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureAccountsTable(ctx); err != nil {
		t.Fatal(err)
	}
	chart := &metadata.ChartOfAccounts{Name: "Основной", Accounts: []metadata.Account{
		{Code: "41", Kind: "active"}, {Code: "60", Kind: "passive"}, {Code: "51", Kind: "active"},
		{Code: "62", Kind: "passive"}, {Code: "90", Kind: "passive"}, {Code: "10", Kind: "active"},
	}}
	if err := db.SyncAccounts(ctx, []*metadata.ChartOfAccounts{chart}); err != nil {
		t.Fatal(err)
	}
	write := func(p time.Time, rows []map[string]any) {
		if err := db.WriteAccountMovements(ctx, ar.Name, "Док", uuid.New(), rows, ar, &p); err != nil {
			t.Fatal(err)
		}
	}
	write(time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC), []map[string]any{{"счётдт": "41", "счёткт": "60", "сумма": float64(50)}})
	write(time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC), []map[string]any{{"счётдт": "41", "счёткт": "60", "сумма": float64(100)}})
	write(time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), []map[string]any{
		{"счётдт": "41", "счёткт": "60", "сумма": float64(1000)}, {"счётдт": "51", "счёткт": "62", "сумма": float64(700)}})
	write(time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC), []map[string]any{
		{"счётдт": "41", "счёткт": "60", "сумма": float64(300)}, {"счётдт": "62", "счёткт": "90", "сумма": float64(700)}})
	write(time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC), []map[string]any{{"счётдт": "10", "счёткт": "60", "сумма": float64(999)}})

	type dtkt = [2]float64
	run := func(totals bool, src string, params map[string]any, wantItogi bool) map[string]dtkt {
		reg := *ar
		reg.Totals = metadata.RegisterTotals{Enabled: totals}
		res, err := query.Compile(src, query.CompileOpts{
			Dialect: db.Dialect(), AccountRegs: []*metadata.AccountRegister{&reg}, Params: params,
		})
		if err != nil {
			t.Fatalf("compile totals=%v: %v", totals, err)
		}
		if totals {
			if uses := strings.Contains(res.SQL, "итоги_акк_"); uses != wantItogi {
				t.Fatalf("ждали чтение итогов=%v, в SQL=%v: %s", wantItogi, uses, res.SQL)
			}
		}
		rows, err := db.Query(ctx, res.SQL, res.Args...)
		if err != nil {
			t.Fatalf("exec totals=%v: %v\nSQL: %s\nArgs: %v", totals, err, res.SQL, res.Args)
		}
		defer rows.Close()
		out := map[string]dtkt{}
		for rows.Next() {
			var acc string
			var dt, kt float64
			if err := rows.Scan(&acc, &dt, &kt); err != nil {
				t.Fatal(err)
			}
			out[acc] = dtkt{dt, kt}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}

	sel := "ВЫБРАТЬ Счёт, Сумма_Дт, Сумма_Кт ИЗ РегистрБухгалтерии.БухОбороты."
	cases := []struct {
		name      string
		src       string
		params    map[string]any
		wantItogi bool
	}{
		{"весь период", sel + "Обороты()", nil, true},
		{"один месяц (июнь)", sel + "Обороты(&Н, &К)", map[string]any{
			"Н": time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), "К": time.Date(2026, 6, 30, 23, 0, 0, 0, time.UTC)}, false},
		{"диапазон через полный средний месяц", sel + "Обороты(&Н, &К)", map[string]any{
			"Н": time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), "К": time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fast := run(true, c.src, c.params, c.wantItogi)
			slow := run(false, c.src, c.params, false)
			if len(fast) != len(slow) {
				t.Fatalf("число счетов: итоги=%d, на лету=%d\n%v\n%v", len(fast), len(slow), fast, slow)
			}
			for acc, v := range slow {
				if fast[acc] != v {
					t.Errorf("счёт %s: обороты итоги=%v, на лету=%v", acc, fast[acc], v)
				}
			}
		})
	}
}

// BenchmarkAccountBalancesTotals сравнивает текущие остатки бухрегистра быстрым
// путём (итоги) и расчётом на лету на регистре с историей проводок. Ожидаем
// сублинейность итогов: они читают ~счета×месяцы строк вместо всей истории.
func BenchmarkAccountBalancesTotals(b *testing.B) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(b.TempDir(), "t.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	ar := &metadata.AccountRegister{
		Name: "БухБенч", Accounts: "Основной",
		Resources: []metadata.Field{{Name: "Сумма", Type: "number"}},
		Totals:    metadata.RegisterTotals{Enabled: true},
	}
	if err := db.MigrateAccountRegisters(ctx, []*metadata.AccountRegister{ar}); err != nil {
		b.Fatal(err)
	}
	if err := db.EnsureAccountsTable(ctx); err != nil {
		b.Fatal(err)
	}
	codes := []string{"41", "60", "51", "62", "90", "10", "20", "26", "44", "68"}
	var accs []metadata.Account
	for _, c := range codes {
		accs = append(accs, metadata.Account{Code: c, Kind: "active"})
	}
	if err := db.SyncAccounts(ctx, []*metadata.ChartOfAccounts{{Name: "Основной", Accounts: accs}}); err != nil {
		b.Fatal(err)
	}

	// ~12k проводок за 24 месяца прямыми вставками, затем один пересчёт итогов.
	const months, perMonth = 24, 500
	start := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		for m := 0; m < months; m++ {
			p := start.AddDate(0, m, 0)
			for i := 0; i < perMonth; i++ {
				if _, err := db.Exec(ctx,
					"INSERT INTO акк_бухбенч (id, period, регистратор, регистратор_тип, счётдт, счёткт, сумма) VALUES (?,?,?,?,?,?,?)",
					uuid.New().String(), p, uuid.New().String(), "Док",
					codes[i%len(codes)], codes[(i+1)%len(codes)], float64(100+i)); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		b.Fatal(err)
	}
	if err := db.RecalcAccountRegisterTotals(ctx, ar); err != nil {
		b.Fatal(err)
	}

	compile := func(totals bool) (string, []any) {
		reg := *ar
		reg.Totals = metadata.RegisterTotals{Enabled: totals}
		res, err := query.Compile("ВЫБРАТЬ Счёт, СуммаОстаток ИЗ РегистрБухгалтерии.БухБенч.Остатки()",
			query.CompileOpts{Dialect: db.Dialect(), AccountRegs: []*metadata.AccountRegister{&reg}})
		if err != nil {
			b.Fatal(err)
		}
		return res.SQL, res.Args
	}
	drain := func(b *testing.B, sql string, args []any) {
		rows, err := db.Query(ctx, sql, args...)
		if err != nil {
			b.Fatal(err)
		}
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			b.Fatal(err)
		}
		rows.Close()
	}
	for _, tc := range []struct {
		name   string
		totals bool
	}{{"on_the_fly", false}, {"totals", true}} {
		sql, args := compile(tc.totals)
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				drain(b, sql, args)
			}
		})
	}
}
