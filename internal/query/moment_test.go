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

// momentValue реализует интерфейс query.momentTimeValue для теста —
// без импорта runtime (избегаем цикла).
type momentValue struct {
	period time.Time
	docID  string
}

func (m *momentValue) PointInTime() (time.Time, string) { return m.period, m.docID }

// Исполняющий тест: момент-временной запрос реально выполняется на SQLite.
// Именно этого не хватало раньше — старый тест проверял только len(Args)
// и подстроки, не выполняя SQL, поэтому рассинхрон плейсхолдеров и
// аргументов («period < ? OR period = ?» с одним arg) проскочил.
func TestMomentTime_ExecutesOnSQLite(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	reg := &metadata.Register{
		Name:       "ОстаткиТоваров",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
	}
	if err := db.MigrateRegisters(ctx, []*metadata.Register{reg}); err != nil {
		t.Fatal(err)
	}

	// Документ A (раньше) — приход 100. Документ B (позже, наш «текущий») —
	// приход 50. Момент времени = период B, docID = B → B исключается,
	// остаток должен быть 100 (только от A).
	docA := uuid.New()
	docB := uuid.New()
	pA := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	pB := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)

	if err := db.WriteMovements(ctx, "ОстаткиТоваров", "ПоступлениеТоваров", docA,
		[]map[string]any{{"ВидДвижения": "Приход", "Номенклатура": "Тумбочка", "Количество": float64(100)}},
		reg, &pA); err != nil {
		t.Fatal(err)
	}
	if err := db.WriteMovements(ctx, "ОстаткиТоваров", "ПоступлениеТоваров", docB,
		[]map[string]any{{"ВидДвижения": "Приход", "Номенклатура": "Тумбочка", "Количество": float64(50)}},
		reg, &pB); err != nil {
		t.Fatal(err)
	}

	mt := &momentValue{period: pB, docID: docB.String()}
	r, err := query.Compile(
		`ВЫБРАТЬ Номенклатура, КоличествоОстаток ИЗ РегистрНакопления.ОстаткиТоваров.Остатки(&МВ)`,
		query.CompileOpts{
			Registers: []*metadata.Register{reg},
			Params:    map[string]any{"МВ": mt},
			Dialect:   storage.SQLiteDialect{},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Главное: запрос выполняется без «missing argument» и т.п.
	rows, err := db.Query(ctx, r.SQL, r.Args...)
	if err != nil {
		t.Fatalf("query execution failed (баг плейсхолдеров?): %v\nSQL: %s\nArgs: %v", err, r.SQL, r.Args)
	}
	defer rows.Close()

	var total float64
	var found bool
	for rows.Next() {
		var name string
		var qty float64
		if err := rows.Scan(&name, &qty); err != nil {
			t.Fatal(err)
		}
		found = true
		total += qty
	}
	if !found {
		t.Fatal("нет строк — момент времени отфильтровал лишнее")
	}
	if total != 100 {
		t.Errorf("остаток на момент B (исключая сам B) = %v, ожидалось 100", total)
	}
}

// .Остатки(МоментВремени) должна давать period < @ OR (period = @ AND recorder != @doc).
func TestCompile_MomentTime_Balances(t *testing.T) {
	src := `ВЫБРАТЬ Номенклатура, КоличествоОстаток
ИЗ РегистрНакопления.ОстаткиТоваров.Остатки(&МВ)`
	reg := &metadata.Register{
		Name: "ОстаткиТоваров",
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: "string"},
		},
		Resources: []metadata.Field{
			{Name: "Количество", Type: "number"},
		},
	}
	mt := &momentValue{
		period: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
		docID:  "11111111-1111-1111-1111-111111111111",
	}
	r, err := query.Compile(src, query.CompileOpts{
		Registers: []*metadata.Register{reg},
		Params:    map[string]any{"МВ": mt},
		Dialect:   storage.SQLiteDialect{},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Должно быть условие на period < и recorder !=
	if !strings.Contains(r.SQL, "period <") || !strings.Contains(r.SQL, "recorder !=") {
		t.Errorf("ожидалось «period < ... AND recorder != ...» в SQL:\n%s", r.SQL)
	}
	// 3 args: period (для «period <») + period (для «period =») + docID.
	// period дублируется, т.к. SQLite-плейсхолдеры анонимные ('?') и каждый
	// '?' требует свой позиционный аргумент.
	if len(r.Args) != 3 {
		t.Errorf("ожидалось 3 args, получили %d: %v", len(r.Args), r.Args)
	}
}

// МоментВремени без docID — упрощённое period <= ...
func TestCompile_MomentTime_NoDocFallback(t *testing.T) {
	src := `ВЫБРАТЬ Номенклатура, КоличествоОстаток
ИЗ РегистрНакопления.ОстаткиТоваров.Остатки(&МВ)`
	reg := &metadata.Register{
		Name:       "ОстаткиТоваров",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: "string"}},
		Resources:  []metadata.Field{{Name: "Количество", Type: "number"}},
	}
	mt := &momentValue{
		period: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
		docID:  "", // нет документа
	}
	r, err := query.Compile(src, query.CompileOpts{
		Registers: []*metadata.Register{reg},
		Params:    map[string]any{"МВ": mt},
		Dialect:   storage.SQLiteDialect{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "period <= ") {
		t.Errorf("без docID ожидалось period <= ...: %s", r.SQL)
	}
	if strings.Contains(r.SQL, "recorder") {
		t.Errorf("без docID recorder не должен упоминаться: %s", r.SQL)
	}
}

// Обычный параметр-дата (не moment) работает как раньше: period <= ...
func TestCompile_PlainDate_StillWorks(t *testing.T) {
	src := `ВЫБРАТЬ Номенклатура, КоличествоОстаток
ИЗ РегистрНакопления.ОстаткиТоваров.Остатки(&Дата)`
	reg := &metadata.Register{
		Name:       "ОстаткиТоваров",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: "string"}},
		Resources:  []metadata.Field{{Name: "Количество", Type: "number"}},
	}
	r, err := query.Compile(src, query.CompileOpts{
		Registers: []*metadata.Register{reg},
		Params:    map[string]any{"Дата": "2026-05-20"},
		Dialect:   storage.SQLiteDialect{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "period <= ") {
		t.Errorf("plain date: ожидалось period <= ...: %s", r.SQL)
	}
	if strings.Contains(r.SQL, "recorder") {
		t.Errorf("plain date: recorder не должен упоминаться: %s", r.SQL)
	}
}

// МоментВремени для info-регистра — берёт только Period, без recorder
// (info-reg не имеет recorder колонки в этом контексте).
func TestCompile_MomentTime_InfoSlice(t *testing.T) {
	src := `ВЫБРАТЬ Цена ИЗ РегистрСведений.ЦеныНоменклатуры.СрезПоследних(&МВ)`
	ir := &metadata.InfoRegister{
		Name:     "ЦеныНоменклатуры",
		Periodic: true,
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: "string"},
		},
		Resources: []metadata.Field{
			{Name: "Цена", Type: "number"},
		},
	}
	mt := &momentValue{
		period: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
		docID:  uuid.New().String(),
	}
	r, err := query.Compile(src, query.CompileOpts{
		InfoRegs: []*metadata.InfoRegister{ir},
		Params:   map[string]any{"МВ": mt},
		Dialect:  storage.SQLiteDialect{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.SQL, "period <=") {
		t.Errorf("info-slice ожидалось period <= ...: %s", r.SQL)
	}
	// recorder для info-reg не нужен
	if strings.Contains(r.SQL, "recorder !=") {
		t.Errorf("info-slice не должен использовать recorder: %s", r.SQL)
	}
}
