package launcher

import (
	"context"
	"html/template"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/storage"
)

// cfgAuthDBs caches storage.DB per base key so we don't open a new connection
// on every configurator request. Key: base.ID (or DSN/path for legacy paths).
var cfgAuthDBs sync.Map // map[string]*storage.DB

// getAuthDB opens (or returns cached) storage.DB for the given base, routing
// by DBType (postgres/sqlite). Cache key is the base ID, which is stable.
func getAuthDB(ctx context.Context, b *Base) (*storage.DB, error) {
	key := b.ID
	if v, ok := cfgAuthDBs.Load(key); ok {
		return v.(*storage.DB), nil
	}
	db, err := OpenDB(ctx, b)
	if err != nil {
		return nil, err
	}
	if actual, loaded := cfgAuthDBs.LoadOrStore(key, db); loaded {
		db.Close()
		return actual.(*storage.DB), nil
	}
	return db, nil
}

func CloseAuthPools() {
	cfgAuthDBs.Range(func(key, value any) bool {
		value.(*storage.DB).Close()
		cfgAuthDBs.Delete(key)
		return true
	})
}

var cfgLoginTmpl = template.Must(template.New("cfg-login").Parse(`<!DOCTYPE html>
<html lang="ru">
<head><meta charset="utf-8"><title>Конфигуратор — Вход</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',Arial,sans-serif;background:#ECE9D8;display:flex;align-items:center;justify-content:center;height:100vh}
.box{background:#fff;padding:32px 40px;border:1px solid #ACA899;border-radius:2px;width:360px;box-shadow:0 2px 8px rgba(0,0,0,.12)}
h2{margin:0 0 6px;color:#1a5fa8;font-size:17px;font-weight:600}
.sub{font-size:12px;color:#666;margin-bottom:20px}
label{display:block;font-size:12px;margin-bottom:3px;color:#444;font-weight:600}
input{width:100%;padding:7px 9px;border:1px solid #ACA899;border-radius:2px;font-size:13px;margin-bottom:14px;outline:none}
input:focus{border-color:#3070D8;box-shadow:0 0 0 2px rgba(48,112,216,.15)}
.btn{width:100%;background:#1a5fa8;color:#fff;border:1px solid #1a5fa8;padding:8px;font-size:13px;border-radius:2px;cursor:pointer;font-weight:500}
.btn:hover{background:#1550a0}
.err{color:#c00;font-size:12px;margin-bottom:12px;padding:7px;background:#fff0f0;border-radius:2px;border:1px solid #fcc}
.back{display:block;margin-top:14px;font-size:12px;color:#1a5fa8;text-decoration:none}
</style></head>
<body>
<div class="box">
  <h2>Конфигуратор — Вход</h2>
  <div class="sub">Только для администраторов</div>
  {{if .Error}}<div class="err">{{.Error}}</div>{{end}}
  <form method="POST">
    <label>Имя пользователя</label>
    <input name="login" autofocus autocomplete="username">
    <label>Пароль</label>
    <input name="password" type="password" autocomplete="current-password">
    <button class="btn" type="submit">Войти</button>
  </form>
  <a class="back" href="/">← Назад к списку баз</a>
</div>
</body></html>`))

func (h *handler) cfgLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	cfgLoginTmpl.Execute(w, map[string]any{"Error": ""})
}

func (h *handler) cfgLoginSubmit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	b, err := h.store.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	r.ParseForm()
	login := r.FormValue("login")
	password := r.FormValue("password")

	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(500)
		cfgLoginTmpl.Execute(w, map[string]any{"Error": "Ошибка подключения к БД: " + err.Error()})
		return
	}

	repo := auth.NewRepo(db)
	if err := repo.EnsureSchema(r.Context()); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(500)
		cfgLoginTmpl.Execute(w, map[string]any{"Error": "Ошибка инициализации: " + err.Error()})
		return
	}

	user, err := repo.Authenticate(r.Context(), login, password)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(401)
		cfgLoginTmpl.Execute(w, map[string]any{"Error": "Неверное имя пользователя или пароль"})
		return
	}

	if !user.IsAdmin {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(403)
		cfgLoginTmpl.Execute(w, map[string]any{"Error": "Доступ запрещён. Только для администраторов."})
		return
	}

	token, err := repo.CreateSession(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "onebase_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/bases/"+id+"/configurator", http.StatusFound)
}

func (h *handler) cfgAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		b, err := h.store.Get(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		db, err := getAuthDB(r.Context(), b)
		if err != nil {
			// Cannot connect to DB — let request through (base may not exist yet)
			next.ServeHTTP(w, r)
			return
		}

		repo := auth.NewRepo(db)
		if err := repo.EnsureSchema(r.Context()); err != nil {
			next.ServeHTTP(w, r)
			return
		}

		hasUsers, err := repo.HasUsers(r.Context())
		if err != nil || !hasUsers {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("onebase_session")
		if err != nil {
			http.Redirect(w, r, "/bases/"+id+"/configurator/login", http.StatusFound)
			return
		}

		user, err := repo.LookupSession(r.Context(), cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/bases/"+id+"/configurator/login", http.StatusFound)
			return
		}

		if !user.IsAdmin {
			http.Redirect(w, r, "/bases/"+id+"/configurator/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}
