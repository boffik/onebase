package extform

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/report"
	"github.com/ivantit66/onebase/internal/storage"
)

// ReportRecord — одна запись внешнего отчёта (таблица _ext_reports).
type ReportRecord struct {
	ID         string
	Name       string
	Content    []byte // «голый» YAML report.Report
	Enabled    bool
	Author     string
	Version    string
	UploadedBy string
	UploadedAt time.Time
}

type ReportRepo struct {
	db *storage.DB
}

func NewReports(db *storage.DB) *ReportRepo {
	return &ReportRepo{db: db}
}

func (r *ReportRepo) EnsureSchema(ctx context.Context) error {
	d := r.db.Dialect()
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS _ext_reports (
			id          %s PRIMARY KEY,
			name        %s NOT NULL UNIQUE,
			content     %s NOT NULL,
			enabled     %s NOT NULL,
			author      %s,
			version     %s,
			uploaded_by %s,
			uploaded_at %s NOT NULL DEFAULT %s
		)`,
		d.TypeText(), d.TypeText(), d.TypeBytes(),
		d.TypeBool(),
		d.TypeText(), d.TypeText(), d.TypeText(),
		d.TypeTimestamp(), d.CurrentTimestampTZ()))
	if err != nil {
		return fmt.Errorf("extform: create _ext_reports: %w", err)
	}
	return nil
}

func (r *ReportRepo) Save(ctx context.Context, rec *ReportRecord) error {
	d := r.db.Dialect()
	if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO _ext_reports (id, name, content, enabled, author, version, uploaded_by, uploaded_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT (name) DO UPDATE SET
			content = EXCLUDED.content,
			enabled = EXCLUDED.enabled,
			author = EXCLUDED.author,
			version = EXCLUDED.version,
			uploaded_by = EXCLUDED.uploaded_by,
			uploaded_at = %s`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3), d.Placeholder(4),
		d.Placeholder(5), d.Placeholder(6), d.Placeholder(7), d.Now(),
		d.Now()),
		rec.ID, rec.Name, rec.Content, true, rec.Author, rec.Version, rec.UploadedBy)
	if err != nil {
		return fmt.Errorf("extform: save report: %w", err)
	}
	return nil
}

func (r *ReportRepo) List(ctx context.Context) ([]*ReportRecord, error) {
	return r.query(ctx, "")
}

func (r *ReportRepo) ListEnabled(ctx context.Context) ([]*ReportRecord, error) {
	return r.query(ctx, " WHERE enabled = "+r.db.Dialect().Placeholder(1), true)
}

func (r *ReportRepo) query(ctx context.Context, where string, args ...any) ([]*ReportRecord, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, content, enabled, author, version, uploaded_by, uploaded_at FROM _ext_reports`+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, fmt.Errorf("extform: list reports: %w", err)
	}
	defer rows.Close()
	var out []*ReportRecord
	for rows.Next() {
		rec, err := scanReportRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *ReportRepo) Get(ctx context.Context, id string) (*ReportRecord, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, content, enabled, author, version, uploaded_by, uploaded_at FROM _ext_reports WHERE id = `+r.db.Dialect().Placeholder(1), id)
	if err != nil {
		return nil, fmt.Errorf("extform: get report: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("extform: отчёт не найден: %s", id)
	}
	return scanReportRecord(rows)
}

func (r *ReportRepo) SetEnabled(ctx context.Context, id string, enabled bool) error {
	d := r.db.Dialect()
	_, err := r.db.Exec(ctx,
		`UPDATE _ext_reports SET enabled = `+d.Placeholder(1)+` WHERE id = `+d.Placeholder(2),
		enabled, id)
	if err != nil {
		return fmt.Errorf("extform: set report enabled: %w", err)
	}
	return nil
}

func (r *ReportRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM _ext_reports WHERE id = `+r.db.Dialect().Placeholder(1), id)
	if err != nil {
		return fmt.Errorf("extform: delete report: %w", err)
	}
	return nil
}

// LoadEnabledReports читает включённые отчёты и парсит их в report.Report для
// регистрации в реестре (reg.SetExternalReports). Битые записи пропускаются.
func (r *ReportRepo) LoadEnabledReports(ctx context.Context) ([]*report.Report, error) {
	recs, err := r.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	var out []*report.Report
	for _, rec := range recs {
		rep, err := report.ParseBytes(rec.Content)
		if err != nil {
			fmt.Printf("extform: пропускаю отчёт %s: %v\n", rec.Name, err)
			continue
		}
		if rep.Name == "" {
			rep.Name = rec.Name
		}
		out = append(out, rep)
	}
	return out, nil
}

func scanReportRecord(rows rowScanner) (*ReportRecord, error) {
	var rec ReportRecord
	var enabled, author, version, uploadedBy, uploadedAt any
	if err := rows.Scan(&rec.ID, &rec.Name, &rec.Content, &enabled, &author, &version, &uploadedBy, &uploadedAt); err != nil {
		return nil, err
	}
	rec.Enabled = scanBool(enabled)
	rec.Author = scanString(author)
	rec.Version = scanString(version)
	rec.UploadedBy = scanString(uploadedBy)
	rec.UploadedAt = storage.ParseDBTime(uploadedAt)
	return &rec, nil
}
