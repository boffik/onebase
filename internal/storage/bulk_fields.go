package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// GetFieldsByIDs reads selected object fields for a batch of object IDs.
// Field columns come from metadata, so callers can accept user-facing field
// names without embedding unchecked identifiers into SQL.
func (db *DB) GetFieldsByIDs(ctx context.Context, entity *metadata.Entity, ids []uuid.UUID, fields []metadata.Field) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any, len(ids))
	if entity == nil || len(ids) == 0 {
		return result, nil
	}

	d := db.dialect
	cols := []string{"id"}
	for _, f := range fields {
		cols = append(cols, metadata.ColumnName(f))
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders = append(placeholders, d.Placeholder(i+1))
		args = append(args, idArg(d, id))
	}

	sql := fmt.Sprintf(
		"SELECT %s FROM %s WHERE id IN (%s)",
		strings.Join(cols, ", "),
		metadata.TableName(entity.Name),
		strings.Join(placeholders, ", "),
	)
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("get fields by ids %s: %w", entity.Name, err)
	}
	defer rows.Close()

	for rows.Next() {
		dest := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		idStr := fmt.Sprintf("%v", normalizeValue(dest[0]))
		row := make(map[string]any, len(fields)+1)
		row["id"] = idStr
		for i, f := range fields {
			row[f.Name] = normalizeFieldValue(f, dest[i+1])
		}
		result[idStr] = row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
