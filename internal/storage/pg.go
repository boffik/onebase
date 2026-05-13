package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the database handle abstraction. It carries either a PostgreSQL
// pgxpool.Pool or a SQLite *sql.DB, plus the matching Dialect. All Exec/Query
// methods route to the right backend transparently.
type DB struct {
	pool     *pgxpool.Pool // non-nil for PG
	sqlDB    *sql.DB       // non-nil for SQLite
	filesDir string
	dialect  Dialect
}

func Connect(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("storage: ping: %w", err)
	}
	filesDir := defaultFilesDir(dsn)
	return &DB{pool: pool, filesDir: filesDir, dialect: PgDialect{}}, nil
}

// Dialect returns the SQL dialect for this connection. Use it to build SQL
// that runs identically on PostgreSQL and SQLite.
func (db *DB) Dialect() Dialect { return db.dialect }

// IsSQLite reports whether this is a SQLite-backed connection.
func (db *DB) IsSQLite() bool { return db.sqlDB != nil }

// IsPostgres reports whether this is a PostgreSQL-backed connection.
func (db *DB) IsPostgres() bool { return db.pool != nil }

func defaultFilesDir(dsn string) string {
	cfg, err := pgxpool.ParseConfig(dsn)
	dbName := "default"
	if err == nil && cfg.ConnConfig.Database != "" {
		dbName = cfg.ConnConfig.Database
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".onebase", "files", dbName)
}

func (db *DB) FilesDir() string { return db.filesDir }

func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
	if db.sqlDB != nil {
		_ = db.sqlDB.Close()
	}
}

// Pool returns the underlying pgxpool.Pool. Panics if called on a SQLite
// connection. New code should use db.Exec/Query/QueryRow/BeginTx instead —
// Pool() is kept only for the legacy launcher/configurator paths that still
// build SQL inline against pgx; those will move to the abstract API in later
// rework.
func (db *DB) Pool() *pgxpool.Pool {
	if db.pool == nil {
		panic("storage.DB.Pool() called on SQLite connection — use db.Exec/Query instead")
	}
	return db.pool
}

// EnsureDatabase creates the PostgreSQL database named in dsn if it does not
// exist. It connects via the "postgres" maintenance database to issue
// CREATE DATABASE, so the caller doesn't need to create the DB manually.
func EnsureDatabase(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("storage: parse dsn: %w", err)
	}
	dbName := cfg.ConnConfig.Database
	if dbName == "" || dbName == "postgres" {
		return nil // nothing to create
	}

	// Connect to the maintenance database
	cfg.ConnConfig.Database = "postgres"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("storage: connect to postgres db: %w", err)
	}
	defer pool.Close()

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, dbName,
	).Scan(&exists); err != nil {
		return fmt.Errorf("storage: check db existence: %w", err)
	}
	if exists {
		return nil
	}

	safe := strings.ReplaceAll(dbName, `"`, `""`)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, safe)); err != nil {
		return fmt.Errorf("storage: create database %q: %w", dbName, err)
	}
	return nil
}
