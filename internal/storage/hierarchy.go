package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// WouldCycle returns true if setting newParentID as the parent of id
// would create a cycle in the hierarchy. Recursive CTEs work on both
// PostgreSQL and SQLite (≥3.8.3) — only placeholders differ.
func (db *DB) WouldCycle(ctx context.Context, table string, id, newParentID uuid.UUID) (bool, error) {
	if id == newParentID {
		return true, nil
	}
	d := db.dialect
	query := fmt.Sprintf(`WITH RECURSIVE anc AS (
		SELECT id, parent_id FROM %s WHERE id = %s
		UNION ALL
		SELECT t.id, t.parent_id FROM %s t
		JOIN anc a ON t.id = a.parent_id WHERE a.parent_id IS NOT NULL
	) SELECT EXISTS(SELECT 1 FROM anc WHERE id = %s)`,
		table, d.Placeholder(1), table, d.Placeholder(2))
	var hasCycle bool
	err := db.QueryRow(ctx, query, idArg(d, newParentID), idArg(d, id)).Scan(&hasCycle)
	return hasCycle, err
}

// GetAncestorIDs returns the chain of IDs from root to id (inclusive), ordered root-first.
func (db *DB) GetAncestorIDs(ctx context.Context, table string, id uuid.UUID) ([]uuid.UUID, error) {
	d := db.dialect
	query := fmt.Sprintf(`WITH RECURSIVE anc AS (
		SELECT id, parent_id FROM %s WHERE id = %s
		UNION ALL
		SELECT t.id, t.parent_id FROM %s t
		JOIN anc a ON t.id = a.parent_id WHERE a.parent_id IS NOT NULL
	) SELECT id FROM anc`, table, d.Placeholder(1), table)
	rows, err := db.Query(ctx, query, idArg(d, id))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var uidStr string
		if err := rows.Scan(&uidStr); err != nil {
			continue
		}
		if uid, err := uuid.Parse(uidStr); err == nil {
			ids = append(ids, uid)
		}
	}
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids, rows.Err()
}
