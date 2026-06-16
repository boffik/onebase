package launcher

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/llm"
	"github.com/ivantit66/onebase/internal/storage"
)

// logCfgAI пишет обращение к ИИ из конфигуратора в журнал _ai_audit, если в
// настройках включён log_history. Best-effort: на ответ пользователю не влияет.
func logCfgAI(ctx context.Context, db *storage.DB, cfg llm.Config, login, task, query, response string, resp llm.ChatResponse) {
	if !cfg.LogHistory || db == nil {
		return
	}
	db.LogAIQuery(ctx, storage.AIAuditEntry{
		UserLogin:    login,
		Task:         task,
		Model:        resp.Model,
		Query:        query,
		Response:     response,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	})
}

// cfgLogin возвращает логин текущего пользователя конфигуратора (или "").
func cfgLogin(ctx context.Context) string {
	if u := cfgUserFromContext(ctx); u != nil {
		return u.Login
	}
	return ""
}

// genResponseSummary формирует текст ответа генератора для журнала: пояснение
// модели + список предложенных объектов.
func genResponseSummary(text string, changes []GenChange) string {
	var b strings.Builder
	b.WriteString(text)
	if len(changes) > 0 {
		b.WriteString("\n\nОбъекты:")
		for _, c := range changes {
			b.WriteString("\n- " + c.Kind + ": " + c.Path)
		}
	}
	return b.String()
}

func truncateText(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// renderAIHistory строит HTML-фрагмент таблицы журнала ИИ (для admin-оверлея).
func renderAIHistory(entries []storage.AIAuditEntry) string {
	var b strings.Builder
	b.WriteString(`<div style="padding:16px"><h3 style="margin:0 0 10px;font-size:15px">История ИИ-запросов</h3>`)
	if len(entries) == 0 {
		b.WriteString(`<div style="color:#888;font-size:12px">Журнал пуст. Включите запись в настройках ИИ: <code>"log_history": true</code>.</div></div>`)
		return b.String()
	}
	b.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:12px"><thead><tr style="text-align:left;border-bottom:1px solid #e2e8f0;color:#666">` +
		`<th style="padding:4px">Дата</th><th style="padding:4px">Инструмент</th><th style="padding:4px">Модель</th><th style="padding:4px">Токены</th><th style="padding:4px">Запрос / ответ</th></tr></thead><tbody>`)
	for _, e := range entries {
		fmt.Fprintf(&b, `<tr style="border-bottom:1px solid #f1f5f9;vertical-align:top">`+
			`<td style="padding:4px;white-space:nowrap">%s</td><td style="padding:4px">%s</td><td style="padding:4px">%s</td><td style="padding:4px;white-space:nowrap">%d+%d</td>`+
			`<td style="padding:4px"><details><summary style="cursor:pointer">%s</summary>`+
			`<pre style="white-space:pre-wrap;word-break:break-word;background:#f8fafc;border:1px solid #e2e8f0;border-radius:4px;padding:6px;margin:4px 0">%s</pre></details></td></tr>`,
			e.At.Format("02.01.2006 15:04"), html.EscapeString(e.Task), html.EscapeString(e.Model),
			e.InputTokens, e.OutputTokens, html.EscapeString(truncateText(e.Query, 80)), html.EscapeString(e.Response))
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

// cfgAdminAIHistory — страница «История ИИ» в админ-меню конфигуратора.
func (h *handler) cfgAdminAIHistory(w http.ResponseWriter, r *http.Request) {
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
	entries, err := db.ListAIAudit(r.Context(), 200)
	if err != nil {
		w.Write([]byte(`<div style="padding:16px;color:#c00">` + html.EscapeString(err.Error()) + `</div>`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(renderAIHistory(entries)))
}
