package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// ConnectSQLite opens (or creates) a SQLite database file at the given path
// and applies pragmas that match the project's operational profile:
//   - WAL journal: concurrent readers don't block on a writer.
//   - synchronous=NORMAL: balance durability/perf.
//   - foreign_keys=ON: enforced FK constraints (SQLite default is off).
//   - busy_timeout=5000: short retry window for concurrent writes.
//   - cache_size=-64000: 64 MiB page cache.
//
// filesDir defaults to <home>/.onebase/files/<basename-without-ext>.
func ConnectSQLite(ctx context.Context, dbPath string) (*DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("storage: sqlite: empty database path")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("storage: sqlite: mkdir: %w", err)
	}

	// modernc.org/sqlite uses "sqlite" driver name (not "sqlite3").
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("storage: sqlite: open: %w", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("storage: sqlite: ping: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-64000",
	}
	for _, p := range pragmas {
		if _, err := conn.ExecContext(ctx, p); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("storage: sqlite: %s: %w", p, err)
		}
	}

	filesDir := defaultFilesDirForSQLite(dbPath)
	return &DB{
		sqlDB:    conn,
		filesDir: filesDir,
		dialect:  SQLiteDialect{},
	}, nil
}

func defaultFilesDirForSQLite(dbPath string) string {
	base := filepath.Base(dbPath)
	// strip .db / .sqlite / .sqlite3 if present
	for _, ext := range []string{".db", ".sqlite", ".sqlite3"} {
		if len(base) > len(ext) && base[len(base)-len(ext):] == ext {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".onebase", "files", base)
}
