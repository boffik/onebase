package storage

import (
	"context"
	"fmt"
)

// EnsureSeqTable creates the _sequences table if it does not exist.
func (db *DB) EnsureSeqTable(ctx context.Context) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS _sequences (
			entity_name TEXT PRIMARY KEY,
			last_num    BIGINT NOT NULL DEFAULT 0
		)`)
	return err
}

// NextNum atomically increments and returns the next sequence number for
// the given entity type. Safe under concurrent requests.
func (db *DB) NextNum(ctx context.Context, entityName string) (int64, error) {
	d := db.dialect
	q := fmt.Sprintf(`
		INSERT INTO _sequences (entity_name, last_num) VALUES (%s, 1)
		ON CONFLICT (entity_name) DO UPDATE
			SET last_num = _sequences.last_num + 1
		RETURNING last_num
	`, d.Placeholder(1))
	var n int64
	err := db.QueryRow(ctx, q, entityName).Scan(&n)
	return n, err
}
