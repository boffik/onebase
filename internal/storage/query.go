package storage

import (
	"context"
	"fmt"
)

// QueryAll executes a compiled SQL query and returns rows (without column names).
func (db *DB) QueryAll(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, _, err := db.RunQuery(ctx, sql, args)
	return rows, err
}

// RunQuery executes a compiled SQL query and returns rows with column names.
func (db *DB) RunQuery(ctx context.Context, sql string, args []any) ([]map[string]any, []string, error) {
	rows, cols, _, err := db.RunQueryLimit(ctx, sql, args, 0)
	return rows, cols, err
}

// RunQueryLimit executes a compiled SQL query and reads at most maxRows+1 rows.
// If maxRows <= 0, it behaves like RunQuery. When truncated is true, rows
// contains only the first maxRows rows.
func (db *DB) RunQueryLimit(ctx context.Context, sql string, args []any, maxRows int) ([]map[string]any, []string, bool, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, nil, false, fmt.Errorf("run query: %w", err)
	}
	defer rows.Close()

	cols := rows.FieldNames()

	var result []map[string]any
	truncated := false
	for rows.Next() {
		if maxRows > 0 && len(result) >= maxRows {
			truncated = true
			break
		}
		dest := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, false, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalizeValue(dest[i])
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	return result, cols, truncated, nil
}
