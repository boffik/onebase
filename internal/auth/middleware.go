package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/ivantit66/onebase/internal/storage"
)

type contextKey string

const userKey contextKey = "auth_user"

func (r *Repo) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		hasUsers, err := r.HasUsers(ctx)
		if err != nil || !hasUsers {
			next.ServeHTTP(w, req)
			return
		}

		// Сессия принимается только из cookie. Токен в query (?_tk=) больше
		// не поддерживается: он утекал в stdout-лог (middleware.Logger пишет
		// полный RequestURI), Referer и историю браузера — план 53, этап 1.
		// Конфигуратор передаёт сессию через /auth/bootstrap?code=<одноразовый>.
		var token string
		if cookie, err := req.Cookie("onebase_session"); err == nil {
			token = cookie.Value
		}

		if token == "" {
			redirectToLogin(w, req)
			return
		}

		user, err := r.LookupSession(ctx, token)
		if err != nil {
			redirectToLogin(w, req)
			return
		}

		// Load roles for this user (best-effort — don't fail if table missing yet)
		if roles, err2 := r.GetRolesForUser(ctx, user.ID); err2 == nil {
			user.Roles = roles
		}

		ctx = context.WithValue(ctx, userKey, user)
		// Inject audit user info for storage layer
		ctx = storage.WithAuditUser(ctx, user.ID, user.Login)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func redirectToLogin(w http.ResponseWriter, req *http.Request) {
	if strings.Contains(req.Header.Get("Accept"), "text/html") {
		http.Redirect(w, req, "/login?return="+req.URL.RequestURI(), http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
}

func UserFromContext(ctx context.Context) *User {
	if u, ok := ctx.Value(userKey).(*User); ok {
		return u
	}
	return nil
}

// ContextWithUser возвращает контекст с привязанным пользователем. Симметрично
// UserFromContext (userKey не экспортируется) — используется тестами и кодом,
// которому нужно подменить пользователя запроса.
func ContextWithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}
