package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Attachment represents a file attached to a document or catalog record.
type Attachment struct {
	ID         uuid.UUID `json:"id"`
	OwnerKind  string    `json:"owner_kind"`
	OwnerName  string    `json:"owner_name"`
	OwnerID    uuid.UUID `json:"owner_id"`
	Filename   string    `json:"filename"`
	MimeType   string    `json:"mime_type"`
	SizeBytes  int64     `json:"size_bytes"`
	UploadedAt time.Time `json:"uploaded_at"`
	UploadedBy string    `json:"uploaded_by"`
}

// EnsureAttachmentTable creates the _attachments table if it does not exist.
func (db *DB) EnsureAttachmentTable(ctx context.Context) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS _attachments (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			owner_kind  TEXT NOT NULL,
			owner_name  TEXT NOT NULL,
			owner_id    UUID NOT NULL,
			filename    TEXT NOT NULL,
			mime_type   TEXT NOT NULL DEFAULT '',
			size_bytes  BIGINT NOT NULL DEFAULT 0,
			uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			uploaded_by TEXT NOT NULL DEFAULT ''
		)
	`)
	return err
}

// ListAttachments returns all attachments for a given owner.
func (db *DB) ListAttachments(ctx context.Context, ownerKind, ownerName string, ownerID uuid.UUID) ([]Attachment, error) {
	rows, err := db.Query(ctx, `
		SELECT id, owner_kind, owner_name, owner_id, filename, mime_type, size_bytes, uploaded_at, uploaded_by
		FROM _attachments
		WHERE owner_kind=$1 AND owner_name=$2 AND owner_id=$3
		ORDER BY uploaded_at DESC
	`, ownerKind, ownerName, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.OwnerKind, &a.OwnerName, &a.OwnerID, &a.Filename, &a.MimeType, &a.SizeBytes, &a.UploadedAt, &a.UploadedBy); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}

// UploadAttachment saves a file to disk and records metadata in the database.
func (db *DB) UploadAttachment(ctx context.Context, ownerKind, ownerName string, ownerID uuid.UUID, filename, mimeType, uploadedBy string, r io.Reader, maxSizeBytes int64) (Attachment, error) {
	id := uuid.New()
	dir := filepath.Join(db.filesDir, ownerName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Attachment{}, err
	}
	filePath := filepath.Join(dir, id.String())
	f, err := os.Create(filePath)
	if err != nil {
		return Attachment{}, err
	}
	defer f.Close()

	limited := io.LimitReader(r, maxSizeBytes+1)
	n, err := io.Copy(f, limited)
	if err != nil {
		os.Remove(filePath)
		return Attachment{}, err
	}
	if n > maxSizeBytes {
		os.Remove(filePath)
		return Attachment{}, fmt.Errorf("файл превышает максимальный размер %d МБ", maxSizeBytes/(1024*1024))
	}

	var a Attachment
	err = db.QueryRow(ctx, `
		INSERT INTO _attachments (id, owner_kind, owner_name, owner_id, filename, mime_type, size_bytes, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, owner_kind, owner_name, owner_id, filename, mime_type, size_bytes, uploaded_at, uploaded_by
	`, id, ownerKind, ownerName, ownerID, filename, mimeType, n, uploadedBy).Scan(
		&a.ID, &a.OwnerKind, &a.OwnerName, &a.OwnerID, &a.Filename, &a.MimeType, &a.SizeBytes, &a.UploadedAt, &a.UploadedBy,
	)
	if err != nil {
		os.Remove(filePath)
		return Attachment{}, err
	}
	return a, nil
}

// GetAttachment returns attachment metadata by ID.
func (db *DB) GetAttachment(ctx context.Context, id uuid.UUID) (*Attachment, error) {
	var a Attachment
	err := db.QueryRow(ctx, `
		SELECT id, owner_kind, owner_name, owner_id, filename, mime_type, size_bytes, uploaded_at, uploaded_by
		FROM _attachments WHERE id=$1
	`, id).Scan(&a.ID, &a.OwnerKind, &a.OwnerName, &a.OwnerID, &a.Filename, &a.MimeType, &a.SizeBytes, &a.UploadedAt, &a.UploadedBy)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// OpenAttachment opens the file for a given attachment ID and returns its metadata.
func (db *DB) OpenAttachment(ctx context.Context, id uuid.UUID) (*os.File, *Attachment, error) {
	a, err := db.GetAttachment(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	filePath := filepath.Join(db.filesDir, a.OwnerName, id.String())
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	return f, a, nil
}

// DeleteAttachment removes the file from disk and deletes the database record.
func (db *DB) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	a, err := db.GetAttachment(ctx, id)
	if err != nil {
		return err
	}
	filePath := filepath.Join(db.filesDir, a.OwnerName, id.String())
	os.Remove(filePath) // best effort
	_, err = db.Exec(ctx, `DELETE FROM _attachments WHERE id=$1`, id)
	return err
}
