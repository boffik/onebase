package auth

import (
	"context"
	"time"
)

// hasUsersCached is the auth hot-path variant of HasUsers. Once any user is
// observed it latches true for the process lifetime (Repo.usersExist), so after
// bootstrap protected requests no longer run SELECT count(*) FROM _users. While
// still unlatched (fresh base, no users yet) it queries every time — that window
// carries negligible traffic. See Plans/111-scalability-review.md §3.2.
func (r *Repo) hasUsersCached(ctx context.Context) (bool, error) {
	if r.usersExist.Load() {
		return true, nil
	}
	has, err := r.HasUsers(ctx)
	if err != nil {
		return false, err
	}
	if has {
		r.usersExist.Store(true)
	}
	return has, nil
}

// rolesForUserCached returns the user's roles from a short-TTL cache, loading
// via GetRolesForUser on a miss. The returned slice is shared across requests
// until invalidated or expired — callers must treat it as read-only.
func (r *Repo) rolesForUserCached(ctx context.Context, userID string) ([]*Role, error) {
	now := time.Now()
	r.rolesMu.RLock()
	e, ok := r.rolesCache[userID]
	r.rolesMu.RUnlock()
	if ok && now.Before(e.expires) {
		return e.roles, nil
	}
	roles, err := r.GetRolesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	r.rolesMu.Lock()
	r.rolesCache[userID] = cachedRoles{roles: roles, expires: now.Add(rolesCacheTTL)}
	r.rolesMu.Unlock()
	return roles, nil
}

// invalidateUserRoles drops one user's cached roles. Call after changing that
// user's role assignments (AssignRole/UnassignRole).
func (r *Repo) invalidateUserRoles(userID string) {
	r.rolesMu.Lock()
	delete(r.rolesCache, userID)
	r.rolesMu.Unlock()
}

// invalidateAllRoles clears the whole roles cache. Call after a change that can
// affect many users at once: role permission sync or role deletion.
func (r *Repo) invalidateAllRoles() {
	r.rolesMu.Lock()
	clear(r.rolesCache)
	r.rolesMu.Unlock()
}
