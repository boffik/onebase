package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/ivantit66/onebase/internal/storage"
)

type User struct {
	ID        string
	Login     string
	FullName  string
	IsAdmin   bool
	CreatedAt time.Time
	Roles     []*Role // loaded by middleware after session lookup
}

type Repo struct {
	db *storage.DB
}

// NewRepo wires the auth repository to the storage layer. Internally Exec/
// Query/QueryRow are routed to PostgreSQL or SQLite via the DB abstraction.
func NewRepo(db *storage.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) EnsureSchema(ctx context.Context) error {
	d := r.db.Dialect()
	usersDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS _users (
			id %s PRIMARY KEY,
			login TEXT UNIQUE NOT NULL,
			password_hash %s NOT NULL,
			full_name TEXT NOT NULL DEFAULT '',
			is_admin %s NOT NULL DEFAULT %s,
			created_at %s NOT NULL DEFAULT %s
		)`, d.TypeUUID(), d.TypeBytes(), d.TypeBool(), boolFalseFor(d), d.TypeTimestamp(), d.CurrentTimestampTZ())
	if _, err := r.db.Exec(ctx, usersDDL); err != nil {
		return fmt.Errorf("auth: create _users: %w", err)
	}
	sessionsDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS _sessions (
			token TEXT PRIMARY KEY,
			user_id %s NOT NULL REFERENCES _users(id) ON DELETE CASCADE,
			expires_at %s NOT NULL
		)`, d.TypeUUID(), d.TypeTimestamp())
	if _, err := r.db.Exec(ctx, sessionsDDL); err != nil {
		return fmt.Errorf("auth: create _sessions: %w", err)
	}
	if err := r.EnsureRolesSchema(ctx); err != nil {
		return err
	}
	return nil
}

// boolFalseFor returns "FALSE" for PG and "0" for SQLite, used in DEFAULT clauses.
func boolFalseFor(d storage.Dialect) string {
	if d.Name() == "sqlite" {
		return "0"
	}
	return "FALSE"
}

func (r *Repo) HasUsers(ctx context.Context) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM _users`).Scan(&count)
	return count > 0, err
}

func (r *Repo) List(ctx context.Context) ([]*User, error) {
	rows, err := r.db.Query(ctx, `SELECT id, login, full_name, is_admin, created_at FROM _users ORDER BY login`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Login, &u.FullName, &u.IsAdmin, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *Repo) Create(ctx context.Context, login, password, fullName string, isAdmin bool) (*User, error) {
	d := r.db.Dialect()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	id := uuid.New().String()
	q := fmt.Sprintf(`INSERT INTO _users (id, login, password_hash, full_name, is_admin) VALUES (%s, %s, %s, %s, %s)`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3), d.Placeholder(4), d.Placeholder(5))
	_, err = r.db.Exec(ctx, q, id, login, hash, fullName, isAdmin)
	if err != nil {
		return nil, fmt.Errorf("auth: create user: %w", err)
	}
	return &User{ID: id, Login: login, FullName: fullName, IsAdmin: isAdmin}, nil
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	d := r.db.Dialect()
	q := fmt.Sprintf(`DELETE FROM _users WHERE id = %s`, d.Placeholder(1))
	_, err := r.db.Exec(ctx, q, id)
	return err
}

func (r *Repo) Authenticate(ctx context.Context, login, password string) (*User, error) {
	d := r.db.Dialect()
	u := &User{}
	var hash []byte
	q := fmt.Sprintf(`SELECT id, login, password_hash, full_name, is_admin FROM _users WHERE login = %s`, d.Placeholder(1))
	err := r.db.QueryRow(ctx, q, login).Scan(&u.ID, &u.Login, &hash, &u.FullName, &u.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("auth: user not found")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		return nil, fmt.Errorf("auth: wrong password")
	}
	return u, nil
}

func (r *Repo) CreateSession(ctx context.Context, userID string) (string, error) {
	d := r.db.Dialect()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	expires := time.Now().Add(24 * time.Hour)
	q := fmt.Sprintf(`INSERT INTO _sessions (token, user_id, expires_at) VALUES (%s, %s, %s)`,
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3))
	_, err := r.db.Exec(ctx, q, token, userID, expires)
	return token, err
}

func (r *Repo) LookupSession(ctx context.Context, token string) (*User, error) {
	d := r.db.Dialect()
	u := &User{}
	q := fmt.Sprintf(`
		SELECT u.id, u.login, u.full_name, u.is_admin
		FROM _sessions s JOIN _users u ON u.id = s.user_id
		WHERE s.token = %s AND s.expires_at > %s
	`, d.Placeholder(1), d.Now())
	err := r.db.QueryRow(ctx, q, token).Scan(&u.ID, &u.Login, &u.FullName, &u.IsAdmin)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repo) DeleteSession(ctx context.Context, token string) error {
	d := r.db.Dialect()
	q := fmt.Sprintf(`DELETE FROM _sessions WHERE token = %s`, d.Placeholder(1))
	_, err := r.db.Exec(ctx, q, token)
	return err
}

// SessionInfo describes one active session.
type SessionInfo struct {
	Login     string
	FullName  string
	IsAdmin   bool
	ExpiresAt time.Time
}

// ActiveSessions returns all non-expired sessions with user info.
func (r *Repo) ActiveSessions(ctx context.Context) ([]*SessionInfo, error) {
	d := r.db.Dialect()
	q := fmt.Sprintf(`
		SELECT u.login, u.full_name, u.is_admin, s.expires_at
		FROM _sessions s
		JOIN _users u ON u.id = s.user_id
		WHERE s.expires_at > %s
		ORDER BY u.login, s.expires_at DESC
	`, d.Now())
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []*SessionInfo
	for rows.Next() {
		si := &SessionInfo{}
		if err := rows.Scan(&si.Login, &si.FullName, &si.IsAdmin, &si.ExpiresAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, si)
	}
	return sessions, rows.Err()
}

// KickUser deletes all sessions for the given login (forces re-login).
func (r *Repo) KickUser(ctx context.Context, login string) error {
	d := r.db.Dialect()
	q := fmt.Sprintf(`DELETE FROM _sessions WHERE user_id = (SELECT id FROM _users WHERE login = %s)`,
		d.Placeholder(1))
	_, err := r.db.Exec(ctx, q, login)
	return err
}
