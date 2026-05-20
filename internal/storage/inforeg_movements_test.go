package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// Движения.X.Добавить() для periodic info-регистра
// раньше тихо терялся. После фикса WriteInfoMovements должен пройти
// и в БД появиться запись с recorder/recorder_type.
func TestWriteInfoMovements_Periodic(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ir := &metadata.InfoRegister{
		Name:     "ЦеныНоменклатуры",
		Periodic: true,
		Dimensions: []metadata.Field{
			{Name: "Номенклатура", Type: "string"},
			{Name: "ТипЦен", Type: "string"},
		},
		Resources: []metadata.Field{
			{Name: "Цена", Type: "number"},
		},
	}
	if err := db.MigrateInfoRegisters(ctx, []*metadata.InfoRegister{ir}); err != nil {
		t.Fatal(err)
	}

	docID := uuid.New()
	period := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	rows := []map[string]any{
		{"Номенклатура": "Тумбочка", "ТипЦен": "Закупочная", "Цена": float64(1000)},
		{"Номенклатура": "Тумбочка", "ТипЦен": "Розничная", "Цена": float64(1500)},
	}

	err = db.WriteInfoMovements(ctx, "ЦеныНоменклатуры", "УстановкаЦен", docID, rows, ir, &period)
	if err != nil {
		t.Fatalf("WriteInfoMovements: %v", err)
	}

	// Проверим что строки записались
	var count int
	row := db.QueryRow(ctx, "SELECT COUNT(*) FROM инфо_ценыноменклатуры")
	if err := row.Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("ожидалось 2 строки, получили %d", count)
	}

	// Проверим recorder заполнен
	var recCount int
	r := db.QueryRow(ctx, "SELECT COUNT(*) FROM инфо_ценыноменклатуры WHERE recorder IS NOT NULL")
	if err := r.Scan(&recCount); err != nil {
		t.Fatal(err)
	}
	if recCount != 2 {
		t.Errorf("recorder не проставился во всех строках: %d из 2", recCount)
	}
}

// Перепроведение документа должно удалить старые строки и положить новые.
func TestWriteInfoMovements_RewriteOnSecondCall(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ir := &metadata.InfoRegister{
		Name:     "Курсы",
		Periodic: true,
		Dimensions: []metadata.Field{
			{Name: "Валюта", Type: "string"},
		},
		Resources: []metadata.Field{
			{Name: "Курс", Type: "number"},
		},
	}
	if err := db.MigrateInfoRegisters(ctx, []*metadata.InfoRegister{ir}); err != nil {
		t.Fatal(err)
	}
	docID := uuid.New()
	period := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)

	// первое проведение — две строки
	rows := []map[string]any{
		{"Валюта": "USD", "Курс": float64(95)},
		{"Валюта": "EUR", "Курс": float64(105)},
	}
	if err := db.WriteInfoMovements(ctx, "Курсы", "ВводКурсов", docID, rows, ir, &period); err != nil {
		t.Fatal(err)
	}

	// перепроведение — другая дата, другая валюта (одна вместо двух)
	period2 := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	rows2 := []map[string]any{
		{"Валюта": "CNY", "Курс": float64(13)},
	}
	if err := db.WriteInfoMovements(ctx, "Курсы", "ВводКурсов", docID, rows2, ir, &period2); err != nil {
		t.Fatal(err)
	}

	var count int
	r := db.QueryRow(ctx, "SELECT COUNT(*) FROM инфо_курсы")
	if err := r.Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("после перепроведения ожидалась 1 строка (CNY), получили %d", count)
	}
}

// Если в row явно задан Период — он перекрывает общий период документа.
func TestWriteInfoMovements_RowPeriodOverridesDocPeriod(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ir := &metadata.InfoRegister{
		Name:     "Курсы",
		Periodic: true,
		Dimensions: []metadata.Field{
			{Name: "Валюта", Type: "string"},
		},
		Resources: []metadata.Field{
			{Name: "Курс", Type: "number"},
		},
	}
	if err := db.MigrateInfoRegisters(ctx, []*metadata.InfoRegister{ir}); err != nil {
		t.Fatal(err)
	}

	docPeriod := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	rowPeriod := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC) // важно: другая дата
	rows := []map[string]any{
		{"Период": rowPeriod, "Валюта": "USD", "Курс": float64(100)},
	}
	if err := db.WriteInfoMovements(ctx, "Курсы", "Doc", uuid.New(), rows, ir, &docPeriod); err != nil {
		t.Fatal(err)
	}
	var got string
	r := db.QueryRow(ctx, "SELECT period FROM инфо_курсы LIMIT 1")
	if err := r.Scan(&got); err != nil {
		t.Fatal(err)
	}
	// SQLite хранит TIMESTAMP как text; ожидаем что период — 2026-06-15...
	if got[:10] != "2026-06-15" {
		t.Errorf("ожидался period 2026-06-15..., получили %q", got)
	}
}
