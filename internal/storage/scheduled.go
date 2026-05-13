package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ScheduledRun struct {
	ID         uuid.UUID  `json:"id"`
	JobName    string     `json:"job_name"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Status     string     `json:"status"`
	Output     string     `json:"output,omitempty"`
	Error      string     `json:"error,omitempty"`
	DurationMs int64      `json:"duration_ms,omitempty"`
}

const scheduledRunsDDL = `
CREATE TABLE IF NOT EXISTS _scheduled_runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name     TEXT NOT NULL,
    started_at   TIMESTAMPTZ NOT NULL,
    finished_at  TIMESTAMPTZ,
    status       TEXT NOT NULL,
    output       TEXT,
    error        TEXT,
    duration_ms  INTEGER
);
CREATE INDEX IF NOT EXISTS idx_scheduled_runs_job ON _scheduled_runs (job_name, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_scheduled_runs_at  ON _scheduled_runs (started_at DESC);
`

func (db *DB) EnsureScheduledRunsTable(ctx context.Context) error {
	_, err := db.Exec(ctx, scheduledRunsDDL)
	if err != nil {
		return fmt.Errorf("scheduled runs DDL: %w", err)
	}
	return nil
}

func (db *DB) InsertScheduledRun(ctx context.Context, jobName string, startedAt time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx,
		`INSERT INTO _scheduled_runs (job_name, started_at, status) VALUES ($1, $2, 'running') RETURNING id`,
		jobName, startedAt,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert scheduled run: %w", err)
	}
	return id, nil
}

func (db *DB) UpdateScheduledRun(ctx context.Context, id uuid.UUID, status, output, errText string, durationMs int64) error {
	now := time.Now()
	_, err := db.Exec(ctx,
		`UPDATE _scheduled_runs SET finished_at=$1, status=$2, output=$3, error=$4, duration_ms=$5 WHERE id=$6`,
		now, status, output, errText, durationMs, id,
	)
	return err
}

func (db *DB) ScheduledRuns(ctx context.Context, jobName string, limit int) ([]ScheduledRun, error) {
	var query string
	var args []any
	if jobName != "" {
		query = `SELECT id, job_name, started_at, finished_at, status, output, error, duration_ms
				 FROM _scheduled_runs WHERE job_name=$1 ORDER BY started_at DESC LIMIT $2`
		args = []any{jobName, limit}
	} else {
		query = `SELECT id, job_name, started_at, finished_at, status, output, error, duration_ms
				 FROM _scheduled_runs ORDER BY started_at DESC LIMIT $1`
		args = []any{limit}
	}
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("scheduled runs query: %w", err)
	}
	defer rows.Close()

	var result []ScheduledRun
	for rows.Next() {
		var r ScheduledRun
		var output, errText *string
		var finishedAt *time.Time
		var durationMs *int64
		if err := rows.Scan(&r.ID, &r.JobName, &r.StartedAt, &finishedAt, &r.Status, &output, &errText, &durationMs); err != nil {
			return nil, err
		}
		if output != nil {
			r.Output = *output
		}
		if errText != nil {
			r.Error = *errText
		}
		if finishedAt != nil {
			r.FinishedAt = finishedAt
		}
		if durationMs != nil {
			r.DurationMs = *durationMs
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
