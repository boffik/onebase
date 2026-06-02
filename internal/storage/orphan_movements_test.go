package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// OrphanMovements не должна зависать на единственном SQLite-соединении
// (SetMaxOpenConns(1)). Раньше вложенный COUNT выполнялся при открытом
// внешнем курсоре rows → блокировка на ожидании соединения. Тест также
// проверяет, что осиротевшие движения находятся и удаляются.
func TestOrphanMovements_NoDeadlockDetectAndDelete(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	doc := &metadata.Entity{
		Name: "ПоступлениеТоваров", Kind: metadata.KindDocument, Posting: true,
		Fields: []metadata.Field{{Name: "Номер", Type: metadata.FieldTypeString}},
	}
	reg := &metadata.Register{
		Name:       "ОстаткиТоваров",
		Dimensions: []metadata.Field{{Name: "Номенклатура", Type: metadata.FieldTypeString}},
		Resources:  []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{doc}); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateRegisters(ctx, []*metadata.Register{reg}); err != nil {
		t.Fatal(err)
	}

	// Движение с recorder на несуществующий документ (таблица документа пуста)
	// — это и есть осиротевшее движение.
	if err := db.WriteMovements(ctx, reg.Name, doc.Name, uuid.New(),
		[]map[string]any{{"ВидДвижения": "Приход", "Номенклатура": "Стол", "Количество": float64(5)}},
		reg, nil); err != nil {
		t.Fatal(err)
	}

	regs := []*metadata.Register{reg}
	ents := []*metadata.Entity{doc}

	// Должна вернуться без зависания и найти 1 осиротевшее движение.
	var total int
	for _, s := range db.OrphanMovements(ctx, regs, ents) {
		total += s.Count
	}
	if total != 1 {
		t.Fatalf("ожидалось 1 осиротевшее движение, получили %d", total)
	}

	// Удаление вычищает их, повторное обнаружение пусто.
	if deleted := db.DeleteOrphanMovements(ctx, regs, ents); deleted != 1 {
		t.Errorf("DeleteOrphanMovements: удалено %d, ожидалось 1", deleted)
	}
	if rest := db.OrphanMovements(ctx, regs, ents); len(rest) != 0 {
		t.Errorf("после очистки осиротевших быть не должно, получили %v", rest)
	}
}
