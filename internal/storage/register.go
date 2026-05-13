package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// OrphanStat describes orphaned movements (recorder document no longer exists).
type OrphanStat struct {
	RegisterName string
	RecorderType string
	Count        int
}

// OrphanMovements returns stats about movements whose recorder document no longer exists.
func (db *DB) OrphanMovements(ctx context.Context, registers []*metadata.Register, entities []*metadata.Entity) []OrphanStat {
	d := db.dialect
	entityTable := make(map[string]string, len(entities))
	for _, e := range entities {
		entityTable[strings.ToLower(e.Name)] = metadata.TableName(e.Name)
	}
	var stats []OrphanStat
	for _, reg := range registers {
		table := metadata.RegisterTableName(reg.Name)
		rows, err := db.Query(ctx, fmt.Sprintf(
			"SELECT recorder_type, COUNT(*) FROM %s GROUP BY recorder_type", table))
		if err != nil {
			continue
		}
		for rows.Next() {
			var recType string
			var total int
			rows.Scan(&recType, &total)

			tbl, exists := entityTable[strings.ToLower(recType)]
			var count int
			if !exists {
				count = total
			} else {
				db.QueryRow(ctx, fmt.Sprintf(
					"SELECT COUNT(*) FROM %s WHERE recorder_type = %s AND recorder NOT IN (SELECT id FROM %s)",
					table, d.Placeholder(1), tbl), recType).Scan(&count)
			}
			if count > 0 {
				stats = append(stats, OrphanStat{RegisterName: reg.Name, RecorderType: recType, Count: count})
			}
		}
		rows.Close()
	}
	return stats
}

// DeleteOrphanMovements removes all movements whose recorder document no longer exists.
// Returns total number of deleted rows.
func (db *DB) DeleteOrphanMovements(ctx context.Context, registers []*metadata.Register, entities []*metadata.Entity) int64 {
	entityTable := make(map[string]string, len(entities))
	for _, e := range entities {
		entityTable[strings.ToLower(e.Name)] = metadata.TableName(e.Name)
	}
	var total int64
	for _, reg := range registers {
		table := metadata.RegisterTableName(reg.Name)
		rows, err := db.Query(ctx, fmt.Sprintf(
			"SELECT DISTINCT recorder_type FROM %s", table))
		if err != nil {
			continue
		}
		var types []string
		for rows.Next() {
			var t string
			rows.Scan(&t)
			types = append(types, t)
		}
		rows.Close()

		d := db.dialect
		for _, recType := range types {
			tbl, exists := entityTable[strings.ToLower(recType)]
			var sql string
			if !exists {
				sql = fmt.Sprintf("DELETE FROM %s WHERE recorder_type = %s", table, d.Placeholder(1))
			} else {
				sql = fmt.Sprintf(
					"DELETE FROM %s WHERE recorder_type = %s AND recorder NOT IN (SELECT id FROM %s)",
					table, d.Placeholder(1), tbl)
			}
			if ct, err := db.Exec(ctx, sql, recType); err == nil {
				total += ct.RowsAffected
			}
		}
	}
	return total
}

// WriteMovements replaces all movements for a document in the given register.
func (db *DB) WriteMovements(ctx context.Context, regName, recorderType string, recorderID uuid.UUID, rows []map[string]any, reg *metadata.Register, period *time.Time) error {
	d := db.dialect
	table := metadata.RegisterTableName(regName)

	if err := db.exec(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE recorder = %s AND recorder_type = %s",
			table, d.Placeholder(1), d.Placeholder(2)),
		idArg(d, recorderID), recorderType,
	); err != nil {
		return fmt.Errorf("clear movements %s: %w", regName, err)
	}

	for i, row := range rows {
		vidDvizh := fmt.Sprintf("%v", ciGet(row, "ВидДвижения"))
		if vidDvizh == "" || vidDvizh == "<nil>" {
			vidDvizh = "Приход"
		}
		cols := []string{"id", "recorder", "recorder_type", "line_number", "period", "вид_движения"}
		phs := []string{d.Placeholder(1), d.Placeholder(2), d.Placeholder(3), d.Placeholder(4), d.Placeholder(5), d.Placeholder(6)}
		periodVal := any(time.Now())
		if period != nil {
			periodVal = *period
		}
		args := []any{idArg(d, uuid.New()), idArg(d, recorderID), recorderType, i + 1, periodVal, vidDvizh}
		idx := 7

		allFields := append(append([]metadata.Field{}, reg.Dimensions...), append(reg.Resources, reg.Attributes...)...)
		for _, f := range allFields {
			cols = append(cols, metadata.ColumnName(f))
			phs = append(phs, d.Placeholder(idx))
			v := ciGet(row, f.Name)
			if f.RefEntity != "" {
				if s, ok := v.(string); ok && s != "" {
					if id, err := uuid.Parse(s); err == nil {
						v = idArg(d, id)
					}
				}
			}
			args = append(args, v)
			idx++
		}

		sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(phs, ", "))
		if err := db.exec(ctx, sql, args...); err != nil {
			return fmt.Errorf("write movement %s row %d: %w", regName, i+1, err)
		}
	}
	return nil
}

// GetMovements returns all movement rows for a register, ordered by period and recorder.
func (db *DB) GetMovements(ctx context.Context, regName string, reg *metadata.Register) ([]map[string]any, error) {
	table := metadata.RegisterTableName(regName)
	cols := []string{"recorder", "recorder_type", "line_number", "period", "вид_движения"}
	allFields := append(append([]metadata.Field{}, reg.Dimensions...), append(reg.Resources, reg.Attributes...)...)
	for _, f := range allFields {
		cols = append(cols, metadata.ColumnName(f))
	}
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY period, recorder, line_number", strings.Join(cols, ", "), table)
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get movements %s: %w", regName, err)
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
		row["recorder"] = normalizeValue(dest[0])
		row["recorder_type"] = dest[1]
		row["line_number"] = dest[2]
		if dest[3] != nil {
			if t, ok := dest[3].(time.Time); ok {
				row["period"] = t.Format("02.01.2006")
			} else {
				row["period"] = dest[3]
			}
		}
		row["вид_движения"] = dest[4]
		for i, f := range allFields {
			row[f.Name] = normalizeValue(dest[5+i])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetDocumentMovements returns all register movements for a specific document.
// Returns a map keyed by register name, each value is a slice of movement rows.
func (db *DB) GetDocumentMovements(ctx context.Context, recorderID uuid.UUID, registers []*metadata.Register) (map[string][]map[string]any, error) {
	result := make(map[string][]map[string]any)
	for _, reg := range registers {
		table := metadata.RegisterTableName(reg.Name)
		cols := []string{"line_number", "period", "вид_движения"}
		allFields := append(append([]metadata.Field{}, reg.Dimensions...), append(reg.Resources, reg.Attributes...)...)
		for _, f := range allFields {
			cols = append(cols, metadata.ColumnName(f))
		}
		query := fmt.Sprintf("SELECT %s FROM %s WHERE recorder = %s ORDER BY line_number",
			strings.Join(cols, ", "), table, db.dialect.Placeholder(1))
		rows, err := db.Query(ctx, query, idArg(db.dialect, recorderID))
		if err != nil {
			continue
		}
		var regRows []map[string]any
		for rows.Next() {
			dest := make([]any, len(cols))
			ptrs := make([]any, len(dest))
			for i := range dest {
				ptrs[i] = &dest[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return result, err
			}
			row := make(map[string]any, len(cols))
			row["line_number"] = dest[0]
			if t, ok := dest[1].(time.Time); ok {
				row["period"] = t.Format("02.01.2006")
			} else {
				row["period"] = dest[1]
			}
			row["вид_движения"] = dest[2]
			for i, f := range allFields {
				row[f.Name] = normalizeValue(dest[3+i])
			}
			regRows = append(regRows, row)
		}
		rows.Close()
		if len(regRows) > 0 {
			result[reg.Name] = regRows
		}
	}
	return result, nil
}

// GetBalances returns aggregated balances grouped by dimension fields.
func (db *DB) GetBalances(ctx context.Context, regName string, reg *metadata.Register) ([]map[string]any, error) {
	table := metadata.RegisterTableName(regName)

	var selectParts, groupBy []string
	var dimNames []string
	for _, f := range reg.Dimensions {
		col := metadata.ColumnName(f)
		selectParts = append(selectParts, col)
		groupBy = append(groupBy, col)
		dimNames = append(dimNames, f.Name)
	}
	var resNames []string
	for _, f := range reg.Resources {
		col := metadata.ColumnName(f)
		selectParts = append(selectParts, fmt.Sprintf(
			"SUM(CASE WHEN вид_движения = 'Приход' THEN %s ELSE -%s END) AS %s", col, col, col))
		resNames = append(resNames, f.Name)
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectParts, ", "), table)
	if len(groupBy) > 0 {
		query += " GROUP BY " + strings.Join(groupBy, ", ")
	}
	query += " ORDER BY " + strings.Join(groupBy, ", ")

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get balances %s: %w", regName, err)
	}
	defer rows.Close()

	totalCols := len(reg.Dimensions) + len(reg.Resources)
	var result []map[string]any
	for rows.Next() {
		dest := make([]any, totalCols)
		ptrs := make([]any, totalCols)
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, totalCols)
		for i, name := range dimNames {
			row[name] = normalizeValue(dest[i])
		}
		for i, name := range resNames {
			row[name] = normalizeValue(dest[len(reg.Dimensions)+i])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ciGet does a case-insensitive map lookup (DSL stores keys in lowercase).
func ciGet(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		return v
	}
	low := strings.ToLower(key)
	for k, v := range m {
		if strings.ToLower(k) == low {
			return v
		}
	}
	return nil
}
