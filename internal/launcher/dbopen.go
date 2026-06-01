package launcher

import (
	"context"
	"fmt"
	"os"

	"github.com/ivantit66/onebase/internal/storage"
)

// OpenDB opens a storage.DB for the given base, routing by DBType.
// Defaults to SQLite when db_type is empty and db is empty (backward compat).
func OpenDB(ctx context.Context, b *Base) (*storage.DB, error) {
	switch b.DBType {
	case "sqlite":
		if b.DBPath == "" {
			return nil, fmt.Errorf("launcher: sqlite base %q has empty db_path", b.Name)
		}
		return storage.ConnectSQLite(ctx, b.DBPath)
	case "", "postgres":
		// backward-compat: пустой db_type и пустой db → SQLite
		if b.DB == "" {
			dbPath := b.DBPath
			if dbPath == "" {
				dbPath = os.TempDir() + string(os.PathSeparator) + "onebase_" + b.ID + ".db"
			}
			return storage.ConnectSQLite(ctx, dbPath)
		}
		return storage.Connect(ctx, b.DB)
	default:
		return nil, fmt.Errorf("launcher: unknown db_type %q", b.DBType)
	}
}
