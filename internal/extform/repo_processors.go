package extform

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/processor"
	"github.com/ivantit66/onebase/internal/storage"
)

// ProcessorRecord — одна запись внешней обработки (таблица _ext_processors).
// Content — единый YAML: метаданные обработки (name/params/...) плюс поле
// code с исходником .proc.os.
type ProcessorRecord struct {
	ID         string
	Name       string
	Content    []byte
	Enabled    bool
	Trusted    bool // админ пометил обработку доверенной → видна и запускается всеми
	Author     string
	Version    string
	UploadedBy string
	UploadedAt time.Time
}

type ProcessorRepo struct {
	db *storage.DB
}

func NewProcessors(db *storage.DB) *ProcessorRepo {
	return &ProcessorRepo{db: db}
}

func (r *ProcessorRepo) EnsureSchema(ctx context.Context) error {
	d := r.db.Dialect()
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS _ext_processors (
			id          %s PRIMARY KEY,
			name        %s NOT NULL UNIQUE,
			content     %s NOT NULL,
			enabled     %s NOT NULL,
			trusted     %s NOT NULL,
			author      %s,
			version     %s,
			uploaded_by %s,
			uploaded_at %s NOT NULL DEFAULT %s
		)`,
		d.TypeText(), d.TypeText(), d.TypeBytes(),
		d.TypeBool(), d.TypeBool(),
		d.TypeText(), d.TypeText(), d.TypeText(),
		d.TypeTimestamp(), d.CurrentTimestampTZ()))
	if err != nil {
		return fmt.Errorf("extform: create _ext_processors: %w", err)
	}
	return nil
}

// Save вставляет/обновляет обработку. Признак trusted сохраняется отдельной
// операцией SetTrusted, поэтому при загрузке/обновлении он сбрасывается в
// false — новый код по умолчанию недоверенный (запускает только админ).
func (r *ProcessorRepo) Save(ctx context.Context, rec *ProcessorRecord) error {
	d := r.db.Dialect()
	if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO _ext_processors (id, name, content, enabled, trusted, author, version, uploaded_by, uploaded_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT (name) DO UPDATE SET
			content = EXCLUDED.content,
			enabled = EXCLUDED.enabled,
			trusted = EXCLUDED.trusted,
			author = EXCLUDED.author,
			version = EXCLUDED.version,
			uploaded_by = EXCLUDED.uploaded_by,
			uploaded_at = %s`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3), d.Placeholder(4),
		d.Placeholder(5), d.Placeholder(6), d.Placeholder(7), d.Placeholder(8), d.Now(),
		d.Now()),
		rec.ID, rec.Name, rec.Content, true, false, rec.Author, rec.Version, rec.UploadedBy)
	if err != nil {
		return fmt.Errorf("extform: save processor: %w", err)
	}
	return nil
}

func (r *ProcessorRepo) List(ctx context.Context) ([]*ProcessorRecord, error) {
	return r.query(ctx, "")
}

func (r *ProcessorRepo) ListEnabled(ctx context.Context) ([]*ProcessorRecord, error) {
	return r.query(ctx, " WHERE enabled = "+r.db.Dialect().Placeholder(1), true)
}

func (r *ProcessorRepo) query(ctx context.Context, where string, args ...any) ([]*ProcessorRecord, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, content, enabled, trusted, author, version, uploaded_by, uploaded_at FROM _ext_processors`+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, fmt.Errorf("extform: list processors: %w", err)
	}
	defer rows.Close()
	var out []*ProcessorRecord
	for rows.Next() {
		rec, err := scanProcessorRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *ProcessorRepo) Get(ctx context.Context, id string) (*ProcessorRecord, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, content, enabled, trusted, author, version, uploaded_by, uploaded_at FROM _ext_processors WHERE id = `+r.db.Dialect().Placeholder(1), id)
	if err != nil {
		return nil, fmt.Errorf("extform: get processor: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("extform: обработка не найдена: %s", id)
	}
	return scanProcessorRecord(rows)
}

func (r *ProcessorRepo) SetEnabled(ctx context.Context, id string, enabled bool) error {
	d := r.db.Dialect()
	_, err := r.db.Exec(ctx, `UPDATE _ext_processors SET enabled = `+d.Placeholder(1)+` WHERE id = `+d.Placeholder(2), enabled, id)
	if err != nil {
		return fmt.Errorf("extform: set processor enabled: %w", err)
	}
	return nil
}

func (r *ProcessorRepo) SetTrusted(ctx context.Context, id string, trusted bool) error {
	d := r.db.Dialect()
	_, err := r.db.Exec(ctx, `UPDATE _ext_processors SET trusted = `+d.Placeholder(1)+` WHERE id = `+d.Placeholder(2), trusted, id)
	if err != nil {
		return fmt.Errorf("extform: set processor trusted: %w", err)
	}
	return nil
}

func (r *ProcessorRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM _ext_processors WHERE id = `+r.db.Dialect().Placeholder(1), id)
	if err != nil {
		return fmt.Errorf("extform: delete processor: %w", err)
	}
	return nil
}

// LoadEnabled читает включённые обработки, разбирает метаданные и код. Возвращает
// список Processor и карту «имя обработки → разобранная программа» для
// регистрации в реестре (reg.SetExternalProcessors). Битые записи
// пропускаются с предупреждением.
func (r *ProcessorRepo) LoadEnabled(ctx context.Context) ([]*processor.Processor, map[string]*ast.Program, error) {
	recs, err := r.ListEnabled(ctx)
	if err != nil {
		return nil, nil, err
	}
	var procs []*processor.Processor
	programs := make(map[string]*ast.Program)
	for _, rec := range recs {
		proc, prog, err := ParseProcessorContent(rec.Content)
		if err != nil {
			fmt.Printf("extform: пропускаю обработку %s: %v\n", rec.Name, err)
			continue
		}
		proc.Trusted = rec.Trusted
		procs = append(procs, proc)
		programs[proc.Name] = prog
	}
	return procs, programs, nil
}

func scanProcessorRecord(rows rowScanner) (*ProcessorRecord, error) {
	var rec ProcessorRecord
	var enabled, trusted, author, version, uploadedBy, uploadedAt any
	if err := rows.Scan(&rec.ID, &rec.Name, &rec.Content, &enabled, &trusted, &author, &version, &uploadedBy, &uploadedAt); err != nil {
		return nil, err
	}
	rec.Enabled = scanBool(enabled)
	rec.Trusted = scanBool(trusted)
	rec.Author = scanString(author)
	rec.Version = scanString(version)
	rec.UploadedBy = scanString(uploadedBy)
	rec.UploadedAt = storage.ParseDBTime(uploadedAt)
	return &rec, nil
}
