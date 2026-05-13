package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// EnsureDeletionMark adds deletion_mark column to all entity tables if missing.
func (db *DB) EnsureDeletionMark(ctx context.Context, entities []*metadata.Entity) error {
	d := db.dialect
	typ := d.TypeBool() + " NOT NULL DEFAULT " + boolFalseLit(d)
	for _, e := range entities {
		table := metadata.TableName(e.Name)
		if err := db.AddColumnIfMissing(ctx, table, "deletion_mark", typ); err != nil {
			return fmt.Errorf("ensure deletion_mark %s: %w", e.Name, err)
		}
	}
	return nil
}

// MarkForDeletion sets or clears the deletion_mark flag for a record.
// Returns an error if the record is predefined (_is_predefined = TRUE).
func (db *DB) MarkForDeletion(ctx context.Context, entityName string, id uuid.UUID, mark bool) error {
	d := db.dialect
	table := metadata.TableName(entityName)
	if mark {
		var isPredefined bool
		if err := db.QueryRow(ctx,
			fmt.Sprintf("SELECT _is_predefined FROM %s WHERE id = %s", table, d.Placeholder(1)),
			idArg(d, id),
		).Scan(&isPredefined); err == nil && isPredefined {
			return fmt.Errorf("нельзя пометить предопределённый элемент %s на удаление", entityName)
		}
	}
	return db.exec(ctx,
		fmt.Sprintf("UPDATE %s SET deletion_mark = %s WHERE id = %s",
			table, d.Placeholder(1), d.Placeholder(2)),
		mark, idArg(d, id))
}

// RefInfo describes a referencing record.
type RefInfo struct {
	EntityName string
	FieldName  string
	Count      int
}

// CheckRefs returns all entities/fields that reference the given object.
func (db *DB) CheckRefs(ctx context.Context, entityName string, id uuid.UUID, allEntities []*metadata.Entity) []RefInfo {
	d := db.dialect
	idA := idArg(d, id)
	var refs []RefInfo
	for _, e := range allEntities {
		for _, f := range e.Fields {
			if f.RefEntity != entityName {
				continue
			}
			col := metadata.ColumnName(f)
			var count int
			db.QueryRow(ctx,
				fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s",
					metadata.TableName(e.Name), col, d.Placeholder(1)),
				idA).Scan(&count)
			if count > 0 {
				refs = append(refs, RefInfo{EntityName: e.Name, FieldName: f.Name, Count: count})
			}
		}
		for _, tp := range e.TableParts {
			for _, f := range tp.Fields {
				if f.RefEntity != entityName {
					continue
				}
				col := metadata.ColumnName(f)
				table := metadata.TablePartTableName(e.Name, tp.Name)
				var count int
				db.QueryRow(ctx,
					fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s", table, col, d.Placeholder(1)),
					idA).Scan(&count)
				if count > 0 {
					refs = append(refs, RefInfo{
						EntityName: e.Name + "." + tp.Name,
						FieldName:  f.Name,
						Count:      count,
					})
				}
			}
		}
	}
	return refs
}

// ListMarked returns all records with deletion_mark=true for the given entity.
func (db *DB) ListMarked(ctx context.Context, entityName string, entity *metadata.Entity) ([]map[string]any, error) {
	table := metadata.TableName(entityName)
	cols := []string{"id"}
	for _, f := range entity.Fields {
		cols = append(cols, metadata.ColumnName(f))
	}
	boolTrue := "TRUE"
	if db.dialect.Name() == "sqlite" {
		boolTrue = "1"
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE deletion_mark = %s", strings.Join(cols, ", "), table, boolTrue)
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		dest := make([]any, len(cols))
		ptrs := make([]any, len(dest))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		row["id"] = normalizeValue(dest[0])
		for i, f := range entity.Fields {
			row[f.Name] = normalizeValue(dest[i+1])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
