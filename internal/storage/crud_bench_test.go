package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/metadata"
)

// BenchmarkUpsert_SQLite меряет пропускную способность вставки записей
// справочника через Upsert — базовый путь записи, которым пользуется
// entityservice и REST API. SQLite в tempdir, без внешней инфраструктуры:
// числа сравнимы между прогонами (benchstat), но это не абсолютная оценка
// PostgreSQL — для неё см. бенч проведения и `onebase bench`.
func BenchmarkUpsert_SQLite(b *testing.B) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { db.Close() })

	entity := &metadata.Entity{
		Name: "Контрагент",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "ИНН", Type: metadata.FieldTypeString},
		},
	}
	if err := db.Migrate(ctx, []*metadata.Entity{entity}); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uuid.New()
		fields := map[string]any{
			"наименование": fmt.Sprintf("ООО Контрагент %d", i),
			"инн":          "7701234567",
		}
		if err := db.Upsert(ctx, entity.Name, id, fields, entity); err != nil {
			b.Fatalf("upsert: %v", err)
		}
	}
}
