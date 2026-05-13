package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgxRows wraps pgx.Rows into storage.Rows.
type pgxRows struct {
	r pgx.Rows
}

func (p *pgxRows) Next() bool          { return p.r.Next() }
func (p *pgxRows) Scan(dst ...any) error { return p.r.Scan(dst...) }
func (p *pgxRows) Err() error          { return p.r.Err() }
func (p *pgxRows) Close()              { p.r.Close() }
func (p *pgxRows) FieldNames() []string {
	fds := p.r.FieldDescriptions()
	out := make([]string, len(fds))
	for i, fd := range fds {
		out[i] = string(fd.Name)
	}
	return out
}

// pgxRow wraps pgx.Row into storage.Row.
type pgxRow struct {
	r pgx.Row
}

func (p pgxRow) Scan(dst ...any) error { return p.r.Scan(dst...) }

func cmdTag(tag pgconn.CommandTag, err error) (CommandTag, error) {
	return CommandTag{RowsAffected: tag.RowsAffected()}, err
}

// pgxTx wraps pgx.Tx into storage.Tx.
type pgxTx struct {
	tx pgx.Tx
}

func (t *pgxTx) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	return cmdTag(t.tx.Exec(ctx, sql, args...))
}

func (t *pgxTx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &pgxRows{r: rows}, nil
}

func (t *pgxTx) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return pgxRow{r: t.tx.QueryRow(ctx, sql, args...)}
}

func (t *pgxTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *pgxTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

// unwrapPgxTx extracts the underlying pgx.Tx from a storage.Tx. Used inside
// the storage package while we still rely on pgx directly. Will go away when
// the SQLite driver lands.
func unwrapPgxTx(t Tx) (pgx.Tx, bool) {
	if pt, ok := t.(*pgxTx); ok {
		return pt.tx, true
	}
	return nil, false
}
