package auth

// White-box tests for the auth hot-path caches (Plans/111 §3.2, P0-2). They live
// in package auth to reach the unexported cached accessors and the latch field.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
)

func newCacheTestRepo(t *testing.T) (*Repo, *storage.DB, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	t.Cleanup(db.Close)
	r := NewRepo(db)
	if err := r.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := r.EnsureRolesSchema(ctx); err != nil {
		t.Fatalf("EnsureRolesSchema: %v", err)
	}
	return r, db, ctx
}

func TestHasUsersCachedLatches(t *testing.T) {
	r, db, ctx := newCacheTestRepo(t)

	// Fresh base: no users, latch stays unset so it keeps checking ground truth.
	if has, err := r.hasUsersCached(ctx); err != nil || has {
		t.Fatalf("fresh base: has=%v err=%v, want false/nil", has, err)
	}
	if r.usersExist.Load() {
		t.Fatal("latch must stay false while there are no users")
	}

	if _, err := r.Create(ctx, "ivan", "secret123", "Ivan", true); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if has, err := r.hasUsersCached(ctx); err != nil || !has {
		t.Fatalf("after Create: has=%v err=%v, want true/nil", has, err)
	}
	if !r.usersExist.Load() {
		t.Fatal("latch must be set once users are observed")
	}

	// The latch is authoritative: even if rows vanish (impossible via supported
	// deletes — ErrLastUser guards the last one), the cached answer stays true,
	// while raw HasUsers still reports ground truth because it is uncached.
	if _, err := db.Exec(ctx, `DELETE FROM _users`); err != nil {
		t.Fatalf("raw delete: %v", err)
	}
	if has, _ := r.hasUsersCached(ctx); !has {
		t.Fatal("latched hasUsersCached must stay true")
	}
	if has, _ := r.HasUsers(ctx); has {
		t.Fatal("uncached HasUsers must reflect ground truth (false)")
	}
}

func TestRolesForUserCachedInvalidation(t *testing.T) {
	r, db, ctx := newCacheTestRepo(t)

	user, err := r.Create(ctx, "ivan", "secret123", "Ivan", true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	role := &Role{Name: "manager"}
	if err := r.SyncRoles(ctx, []*Role{role}); err != nil {
		t.Fatalf("SyncRoles: %v", err)
	}

	// Miss → loads 0 roles and caches them.
	if got, err := r.rolesForUserCached(ctx, user.ID); err != nil || len(got) != 0 {
		t.Fatalf("initial roles: got=%d err=%v, want 0/nil", len(got), err)
	}

	// AssignRole invalidates the user's entry → next read reflects the grant.
	if err := r.AssignRole(ctx, user.ID, role.ID); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}
	if got, err := r.rolesForUserCached(ctx, user.ID); err != nil || len(got) != 1 {
		t.Fatalf("after AssignRole: got=%d err=%v, want 1/nil", len(got), err)
	}

	// Prove it actually caches: remove the assignment via raw SQL, bypassing
	// UnassignRole's invalidation. Within the TTL the cached role still shows.
	if _, err := db.Exec(ctx, `DELETE FROM _user_roles WHERE user_id = `+db.Dialect().Placeholder(1), user.ID); err != nil {
		t.Fatalf("raw unassign: %v", err)
	}
	if got, _ := r.rolesForUserCached(ctx, user.ID); len(got) != 1 {
		t.Fatalf("within TTL the cache must still return 1 role, got %d", len(got))
	}

	// Explicit invalidation → reflects the removal immediately.
	r.invalidateAllRoles()
	if got, _ := r.rolesForUserCached(ctx, user.ID); len(got) != 0 {
		t.Fatalf("after invalidateAllRoles: want 0 roles, got %d", len(got))
	}
}
