package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// WriteCatalogRecord upserts a catalog record from DSL code
// (Справочники.X.Создать().Записать()). idStr пустой или невалидный —
// генерируется новый UUID. Возвращает строковый UUID записанной записи.
func (db *DB) WriteCatalogRecord(ctx context.Context, entity *metadata.Entity, idStr string, fields map[string]any) (string, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		id = uuid.New()
	}
	if err := db.Upsert(ctx, entity.Name, id, fields, entity); err != nil {
		return "", err
	}
	return id.String(), nil
}
