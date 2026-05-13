package storage

import (
	"context"

	"github.com/google/uuid"
)

// WouldCycle returns true if setting newParentID as the parent of id
// would create a cycle in the hierarchy.
func (db *DB) WouldCycle(ctx context.Context, table string, id, newParentID uuid.UUID) (bool, error) {
	if id == newParentID {
		return true, nil
	}
	query := `WITH RECURSIVE anc AS (
		SELECT id, parent_id FROM ` + table + ` WHERE id = $1
		UNION ALL
		SELECT t.id, t.parent_id FROM ` + table + ` t
		JOIN anc a ON t.id = a.parent_id WHERE a.parent_id IS NOT NULL
	) SELECT EXISTS(SELECT 1 FROM anc WHERE id = $2)`
	var hasCycle bool
	err := db.QueryRow(ctx, query, newParentID, id).Scan(&hasCycle)
	return hasCycle, err
}

// GetAncestorIDs returns the chain of IDs from root to id (inclusive), ordered root-first.
func (db *DB) GetAncestorIDs(ctx context.Context, table string, id uuid.UUID) ([]uuid.UUID, error) {
	query := `WITH RECURSIVE anc AS (
		SELECT id, parent_id FROM ` + table + ` WHERE id = $1
		UNION ALL
		SELECT t.id, t.parent_id FROM ` + table + ` t
		JOIN anc a ON t.id = a.parent_id WHERE a.parent_id IS NOT NULL
	) SELECT id FROM anc`
	rows, err := db.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			continue
		}
		ids = append(ids, uid)
	}
	// reverse: CTE returns id-first (deepest first), we want root-first
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids, rows.Err()
}
