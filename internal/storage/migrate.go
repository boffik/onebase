package storage

import (
	"context"
	"fmt"
	"unicode"

	"github.com/ivantit66/onebase/internal/metadata"
)

// toSnakeCase converts CamelCase (including Cyrillic) to snake_case.
// Used to detect and rename columns created by older schema versions.
func toSnakeCase(s string) string {
	runes := []rune(s)
	out := make([]rune, 0, len(runes)+4)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			out = append(out, '_')
		}
		out = append(out, unicode.ToLower(r))
	}
	return string(out)
}

// renameSnakeCols renames old snake_case columns (e.g. тип_контрагента)
// to the current lowercase style (типконтрагента) if they exist in the table.
// PG-only: uses information_schema. No-op on SQLite (legacy data isn't a concern there).
func (db *DB) renameSnakeCols(ctx context.Context, table string, fields []metadata.Field) {
	if db.IsSQLite() {
		return
	}
	for _, f := range fields {
		newCol := metadata.ColumnName(f)
		oldCol := toSnakeCase(f.Name)
		if oldCol == newCol {
			continue
		}
		var oldExists bool
		db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2)`,
			table, oldCol).Scan(&oldExists)
		if !oldExists {
			continue
		}
		var newExists bool
		db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2)`,
			table, newCol).Scan(&newExists)
		if newExists {
			db.Exec(ctx, fmt.Sprintf(
				"UPDATE %s SET %s = %s WHERE %s IS NOT NULL AND %s IS NULL",
				table, newCol, oldCol, oldCol, newCol))
			db.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, oldCol))
		} else {
			db.Exec(ctx, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", table, oldCol, newCol))
		}
	}
}

// MigrateRegisters creates register tables.
func (db *DB) MigrateRegisters(ctx context.Context, registers []*metadata.Register) error {
	d := db.dialect
	for _, reg := range registers {
		if _, err := db.Exec(ctx, CreateRegisterSQL(d, reg)); err != nil {
			return fmt.Errorf("migrate register %s: %w", reg.Name, err)
		}
		table := metadata.RegisterTableName(reg.Name)
		if err := db.AddColumnIfMissing(ctx, table, "period", d.TypeTimestamp()); err != nil {
			return fmt.Errorf("migrate register %s.period: %w", reg.Name, err)
		}
		allFields := append(append([]metadata.Field{}, reg.Dimensions...), append(reg.Resources, reg.Attributes...)...)
		db.renameSnakeCols(ctx, table, allFields)
		for _, f := range allFields {
			if err := db.AddColumnIfMissing(ctx, table, metadata.ColumnName(f), fieldType(d, f)); err != nil {
				return fmt.Errorf("migrate register %s.%s: %w", reg.Name, f.Name, err)
			}
		}
	}
	return nil
}

// MigrateInfoRegisters creates tables for info registers.
func (db *DB) MigrateInfoRegisters(ctx context.Context, regs []*metadata.InfoRegister) error {
	d := db.dialect
	for _, ir := range regs {
		if _, err := db.Exec(ctx, CreateInfoRegisterSQL(d, ir)); err != nil {
			return fmt.Errorf("migrate info register %s: %w", ir.Name, err)
		}
		table := metadata.InfoRegTableName(ir.Name)
		if err := db.AddColumnIfMissing(ctx, table, "updated_at", d.TypeTimestamp()); err != nil {
			return fmt.Errorf("migrate info register %s.updated_at: %w", ir.Name, err)
		}
		for _, f := range ir.Resources {
			if err := db.AddColumnIfMissing(ctx, table, metadata.ColumnName(f), fieldType(d, f)); err != nil {
				return fmt.Errorf("migrate info register %s.%s: %w", ir.Name, f.Name, err)
			}
		}
	}
	return nil
}

// Migrate applies CREATE TABLE and ADD COLUMN IF NOT EXISTS for all entities.
func (db *DB) Migrate(ctx context.Context, entities []*metadata.Entity) error {
	d := db.dialect
	if err := db.EnsureSeqTable(ctx); err != nil {
		return fmt.Errorf("migrate: sequences table: %w", err)
	}
	if err := db.EnsureNumeratorSchema(ctx); err != nil {
		return fmt.Errorf("migrate: numerators table: %w", err)
	}
	ordered := orderByDependency(entities)
	for _, e := range ordered {
		if _, err := db.Exec(ctx, CreateTableSQL(d, e)); err != nil {
			return fmt.Errorf("migrate %s: %w", e.Name, err)
		}
		if err := db.EnsurePredefinedColumns(ctx, []*metadata.Entity{e}); err != nil {
			return fmt.Errorf("migrate: predefined columns: %w", err)
		}
		table := metadata.TableName(e.Name)
		if e.Kind == metadata.KindDocument {
			if err := db.AddColumnIfMissing(ctx, table, "posted", d.TypeBool()+" NOT NULL DEFAULT "+boolFalseLit(d)); err != nil {
				return fmt.Errorf("migrate %s.posted: %w", e.Name, err)
			}
		}
		db.renameSnakeCols(ctx, table, e.Fields)
		for _, f := range e.Fields {
			if err := db.AddColumnIfMissing(ctx, table, metadata.ColumnName(f), fieldType(d, f)); err != nil {
				return fmt.Errorf("migrate %s.%s: %w", e.Name, f.Name, err)
			}
		}
		if err := db.AddColumnIfMissing(ctx, table, "deletion_mark", d.TypeBool()+" NOT NULL DEFAULT "+boolFalseLit(d)); err != nil {
			return fmt.Errorf("migrate %s.deletion_mark: %w", e.Name, err)
		}
		if e.Hierarchical {
			if err := db.AddHierarchyColumns(ctx, table); err != nil {
				return fmt.Errorf("migrate %s hierarchy: %w", e.Name, err)
			}
		}
		for _, tp := range e.TableParts {
			if _, err := db.Exec(ctx, CreateTablePartSQL(d, e, tp)); err != nil {
				return fmt.Errorf("migrate %s.%s: %w", e.Name, tp.Name, err)
			}
			tpTable := metadata.TablePartTableName(e.Name, tp.Name)
			for _, f := range tp.Fields {
				if err := db.AddColumnIfMissing(ctx, tpTable, metadata.ColumnName(f), fieldType(d, f)); err != nil {
					return fmt.Errorf("migrate %s.%s.%s: %w", e.Name, tp.Name, f.Name, err)
				}
			}
		}
	}
	for _, e := range ordered {
		if err := db.SyncPredefined(ctx, e); err != nil {
			return fmt.Errorf("migrate: sync predefined %s: %w", e.Name, err)
		}
	}
	return nil
}

// orderByDependency sorts entities so referenced entities come before referencing ones.
func orderByDependency(entities []*metadata.Entity) []*metadata.Entity {
	byName := make(map[string]*metadata.Entity, len(entities))
	for _, e := range entities {
		byName[e.Name] = e
	}
	visited := make(map[string]bool)
	var result []*metadata.Entity
	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		e := byName[name]
		if e == nil {
			return
		}
		for _, f := range e.Fields {
			if f.RefEntity != "" {
				visit(f.RefEntity)
			}
		}
		result = append(result, e)
	}
	for _, e := range entities {
		visit(e.Name)
	}
	return result
}
