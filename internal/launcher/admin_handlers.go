package launcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/version"
)

// ── Admin panel handlers for configurator ────────────────────────────────────

func (h *handler) cfgAdminUsers(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		w.Write([]byte(`<div style="padding:16px;color:#c00">Нет подключения к БД</div>`))
		return
	}
	repo := auth.NewRepo(db)
	repo.EnsureSchema(r.Context())
	users, _ := repo.List(r.Context())

	html := `<div style="padding:16px">
	<div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:14px">
	  <h3 style="margin:0;font-size:15px">Пользователи</h3>
	  <button onclick="cfgUserNew()" style="background:#1a5fa8;color:#fff;border:none;padding:5px 14px;border-radius:3px;cursor:pointer;font-size:12px">+ Добавить</button>
	</div>
	<div id="cfg-user-new" style="display:none;margin-bottom:14px;padding:12px;background:#f8fafc;border:1px solid #e2e8f0;border-radius:4px">
	  <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:end">
	    <div style="flex:1;min-width:120px"><label style="font-size:11px;color:#666">Логин</label><input id="cfg-un" style="width:100%;padding:5px 7px;border:1px solid #ccc;border-radius:3px;font-size:12px"></div>
	    <div style="flex:1;min-width:120px"><label style="font-size:11px;color:#666">Пароль</label><input id="cfg-up" type="password" style="width:100%;padding:5px 7px;border:1px solid #ccc;border-radius:3px;font-size:12px"></div>
	    <div style="flex:1;min-width:120px"><label style="font-size:11px;color:#666">Полное имя</label><input id="cfg-ufn" style="width:100%;padding:5px 7px;border:1px solid #ccc;border-radius:3px;font-size:12px"></div>
	    <label style="font-size:12px;display:flex;align-items:center;gap:4px"><input type="checkbox" id="cfg-ua"> Админ</label>
	    <button onclick="cfgUserCreate()" style="background:#16a34a;color:#fff;border:none;padding:5px 12px;border-radius:3px;cursor:pointer;font-size:12px">Создать</button>
	    <button onclick="document.getElementById('cfg-user-new').style.display='none'" style="background:#e2e8f0;color:#333;border:none;padding:5px 10px;border-radius:3px;cursor:pointer;font-size:12px">Отмена</button>
	  </div>
	  <div id="cfg-user-err" style="color:#c00;font-size:11px;margin-top:6px;display:none"></div>
	</div>
	<table style="width:100%;border-collapse:collapse;font-size:12px">
	<tr style="background:#f1f5f9"><th style="text-align:left;padding:6px 8px;font-weight:600">Логин</th><th style="text-align:left;padding:6px 8px;font-weight:600">Имя</th><th style="text-align:center;padding:6px 8px;font-weight:600">Админ</th><th style="text-align:left;padding:6px 8px;font-weight:600">Создан</th><th style="padding:6px 8px"></th></tr>`
	for i, u := range users {
		bg := ""
		if i%2 == 1 {
			bg = ` style="background:#f9fafb"`
		}
		admin := ""
		if u.IsAdmin {
			admin = `<span style="color:#16a34a;font-weight:600">Да</span>`
		}
		html += fmt.Sprintf(`<tr%s><td style="padding:5px 8px">%s</td><td style="padding:5px 8px">%s</td><td style="padding:5px 8px;text-align:center">%s</td><td style="padding:5px 8px;color:#888">%s</td><td style="padding:5px 8px"><button onclick="cfgUserDel('%s')" style="color:#c00;background:none;border:none;cursor:pointer;font-size:11px" title="Удалить">✕</button></td></tr>`,
			bg, escHTML(u.Login), escHTML(u.FullName), admin, u.CreatedAt.Format("02.01.2006"), u.ID)
	}
	if len(users) == 0 {
		html += `<tr><td colspan="5" style="padding:20px;text-align:center;color:#999">Нет пользователей</td></tr>`
	}
	html += `</table></div>
<script>
function cfgUserNew(){document.getElementById('cfg-user-new').style.display='block';document.getElementById('cfg-un').focus()}
function cfgUserCreate(){
  var d={login:document.getElementById('cfg-un').value,password:document.getElementById('cfg-up').value,fullName:document.getElementById('cfg-ufn').value,isAdmin:document.getElementById('cfg-ua').checked};
  fetch('/bases/` + b.ID + `/configurator/admin/users/create',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(d)})
    .then(function(r){return r.json()}).then(function(r){
      if(r.error){document.getElementById('cfg-user-err').textContent=r.error;document.getElementById('cfg-user-err').style.display='block';return}
      cfgAdmin('users')
    })
}
function cfgUserDel(id){
  if(!confirm('Удалить пользователя?'))return;
  fetch('/bases/` + b.ID + `/configurator/admin/users/delete',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:id})})
    .then(function(){cfgAdmin('users')})
}
</script>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *handler) cfgAdminUserCreate(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
		FullName string `json:"fullName"`
		IsAdmin  bool   `json:"isAdmin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if req.Login == "" || req.Password == "" {
		writeJSON(w, 400, map[string]any{"error": "Логин и пароль обязательны"})
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	repo := auth.NewRepo(db)
	if _, err := repo.Create(r.Context(), req.Login, req.Password, req.FullName, req.IsAdmin); err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (h *handler) cfgAdminUserDelete(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	repo := auth.NewRepo(db)
	if err := repo.Delete(r.Context(), req.ID); err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (h *handler) cfgAdminSessions(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		w.Write([]byte(`<div style="padding:16px;color:#c00">Нет подключения к БД</div>`))
		return
	}
	repo := auth.NewRepo(db)
	repo.EnsureSchema(r.Context())
	sessions, _ := repo.ActiveSessions(r.Context())

	html := `<div style="padding:16px">
	<h3 style="margin:0 0 14px;font-size:15px">Активные пользователи</h3>
	<table style="width:100%;border-collapse:collapse;font-size:12px">
	<tr style="background:#f1f5f9"><th style="text-align:left;padding:6px 8px;font-weight:600">Логин</th><th style="text-align:left;padding:6px 8px;font-weight:600">Имя</th><th style="text-align:center;padding:6px 8px;font-weight:600">Админ</th><th style="text-align:left;padding:6px 8px;font-weight:600">Действует до</th><th style="padding:6px 8px"></th></tr>`
	for i, s := range sessions {
		bg := ""
		if i%2 == 1 {
			bg = ` style="background:#f9fafb"`
		}
		admin := ""
		if s.IsAdmin {
			admin = `<span style="color:#16a34a;font-weight:600">Да</span>`
		}
		html += fmt.Sprintf(`<tr%s><td style="padding:5px 8px">%s</td><td style="padding:5px 8px">%s</td><td style="padding:5px 8px;text-align:center">%s</td><td style="padding:5px 8px;color:#888">%s</td><td style="padding:5px 8px"><button onclick="cfgKick('%s')" style="color:#c00;background:none;border:none;cursor:pointer;font-size:11px" title="Завершить сессию">✕</button></td></tr>`,
			bg, escHTML(s.Login), escHTML(s.FullName), admin, s.ExpiresAt.Format("02.01.2006 15:04"), escHTML(s.Login))
	}
	if len(sessions) == 0 {
		html += `<tr><td colspan="5" style="padding:20px;text-align:center;color:#999">Нет активных сессий</td></tr>`
	}
	html += `</table></div>
<script>
function cfgKick(login){
  if(!confirm('Завершить сессию '+login+'?'))return;
  fetch('/bases/` + b.ID + `/configurator/admin/sessions/kick',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({login:login})})
    .then(function(){cfgAdmin('sessions')})
}
</script>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *handler) cfgAdminSessionKick(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	var req struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	repo := auth.NewRepo(db)
	repo.KickUser(r.Context(), req.Login)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (h *handler) cfgAdminAudit(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		w.Write([]byte(`<div style="padding:16px;color:#c00">Нет подключения к БД</div>`))
		return
	}
	rows, err := db.Query(r.Context(), `
		SELECT user_login, action, entity_kind, entity_name, at
		FROM _audit ORDER BY at DESC LIMIT 100`)
	if err != nil {
		w.Write([]byte(`<div style="padding:16px;color:#999">Журнал регистрации пуст или таблица не создана</div>`))
		return
	}
	defer rows.Close()

	html := `<div style="padding:16px">
	<h3 style="margin:0 0 14px;font-size:15px">Журнал регистрации</h3>
	<table style="width:100%;border-collapse:collapse;font-size:12px">
	<tr style="background:#f1f5f9"><th style="text-align:left;padding:6px 8px;font-weight:600">Время</th><th style="text-align:left;padding:6px 8px;font-weight:600">Пользователь</th><th style="text-align:left;padding:6px 8px;font-weight:600">Действие</th><th style="text-align:left;padding:6px 8px;font-weight:600">Объект</th></tr>`
	i := 0
	for rows.Next() {
		var userLogin, action, kind, entityName string
		var at time.Time
		rows.Scan(&userLogin, &action, &kind, &entityName, &at)
		bg := ""
		if i%2 == 1 {
			bg = ` style="background:#f9fafb"`
		}
		obj := escHTML(entityName)
		if kind != "" {
			obj = escHTML(kind) + ": " + obj
		}
		html += fmt.Sprintf(`<tr%s><td style="padding:5px 8px;color:#888;white-space:nowrap">%s</td><td style="padding:5px 8px">%s</td><td style="padding:5px 8px">%s</td><td style="padding:5px 8px">%s</td></tr>`,
			bg, at.Format("02.01.2006 15:04:05"), escHTML(userLogin), escHTML(action), obj)
		i++
	}
	if i == 0 {
		html += `<tr><td colspan="4" style="padding:20px;text-align:center;color:#999">Журнал пуст</td></tr>`
	}
	html += `</table></div>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *handler) cfgAdminAbout(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	html := fmt.Sprintf(`<div style="padding:24px;max-width:400px">
	<div style="text-align:center;margin-bottom:20px">
	  <div style="font-size:32px;margin-bottom:8px">&#9889;</div>
	  <div style="font-size:18px;font-weight:600;color:#1a5fa8">OneBase</div>
	</div>
	<table style="width:100%%;border-collapse:collapse;font-size:13px">
	<tr><td style="padding:6px 0;color:#888;width:140px">Версия платформы</td><td style="padding:6px 0">%s</td></tr>
	<tr><td style="padding:6px 0;color:#888">Режим конфигурации</td><td style="padding:6px 0">%s</td></tr>
	<tr><td style="padding:6px 0;color:#888">База данных</td><td style="padding:6px 0">%s</td></tr>
	<tr><td style="padding:6px 0;color:#888">Порт</td><td style="padding:6px 0">:%d</td></tr>
	</table>
	</div>`,
		escHTML(version.String()),
		escHTML(b.ConfigSource),
		maskDSN(escHTML(b.DB)),
		b.Port)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func escHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// maskDSN hides password in a connection string.
// postgres://user:secret@host:5432/db → postgres://user:***@host:5432/db
func maskDSN(dsn string) string {
	// URL format: postgres://user:pass@host/db
	if i := strings.Index(dsn, "://"); i >= 0 {
		rest := dsn[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			userPart := rest[:at]
			if colon := strings.LastIndex(userPart, ":"); colon >= 0 {
				return dsn[:i+3+colon+1] + "***" + dsn[i+3+at:]
			}
		}
	}
	// DSN format: host=... password=secret ...
	if i := strings.Index(dsn, "password="); i >= 0 {
		end := i + len("password=")
		rest := dsn[end:]
		if sp := strings.IndexByte(rest, ' '); sp >= 0 {
			return dsn[:end] + "***" + rest[sp:]
		}
		return dsn[:end] + "***"
	}
	return dsn
}
