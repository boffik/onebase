package launcher

import (
	"context"
	"fmt"

	"github.com/ivantit66/onebase/internal/storage"
)

// OpenDB opens a storage.DB for the given base, routing by DBType.
// Defaults to postgres for backward compatibility (older bases have no DBType).
func OpenDB(ctx context.Context, b *Base) (*storage.DB, error) {
	switch b.DBType {
	case "sqlite":
		if b.DBPath == "" {
			return nil, fmt.Errorf("launcher: sqlite base %q has empty db_path", b.Name)
		}
		return storage.ConnectSQLite(ctx, b.DBPath)
	case "", "postgres":
		return storage.Connect(ctx, b.DB)
	default:
		return nil, fmt.Errorf("launcher: unknown db_type %q", b.DBType)
	}
}
