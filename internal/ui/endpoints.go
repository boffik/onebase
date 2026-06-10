package ui

// Входящие REST-эндпоинты /api/hooks/* (план 58): внешняя система (Telegram,
// платёжка, n8n/Make) вызывает именованный DSL-обработчик конфигурации.
// Эндпоинты объявляются в endpoints/*.yaml, обработчик — src/<handler>.endpoint.os
// с процедурой Обработать(Запрос, Ответ). Вместе с исходящими веб-хуками
// (план 29) — полный цикл интеграций.
//
// Безопасность: auth none|token|hmac per-endpoint (секрет из ${env:VAR});
// rate-limit на эндпоинт; тело ограничено MaxBytesReader. Код обработчика
// пишет владелец конфигурации — недоверенными являются только данные запроса
// (строки в DSL); песочница DSL (план 57, этап 0) — отдельный шаг.

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/runtime"
)

// endpointMaxBodyBytes — потолок тела входящего запроса.
const endpointMaxBodyBytes = 10 << 20 // 10 МиБ

// MountEndpoints монтирует catch-all маршрут входящих эндпоинтов. Вызывается
// на корневом роутере (вне auth-группы): аутентификация у эндпоинтов своя
// (token/hmac), а сессии пользователей тут ни при чём.
func (s *Server) MountEndpoints(r chi.Router) {
	r.HandleFunc("/api/hooks/*", s.endpointCall)
}

// endpointLimiter — скользящее окно запросов в минуту на эндпоинт.
// Отдельный от aiWindowLimiter: лимит у каждого эндпоинта свой (rate_limit).
type endpointLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func (l *endpointLimiter) allow(key string, max int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hits == nil {
		l.hits = make(map[string][]time.Time)
	}
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// endpointCall — единая точка входа /api/hooks/*: lookup по path в registry
// (динамический — hot-reload подхватывает новые эндпоинты), auth, rate-limit,
// запуск DSL-обработчика.
func (s *Server) endpointCall(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")
	re := s.reg.GetEndpointByPath(path)
	if re == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown endpoint"})
		return
	}
	m := re.Meta
	if !strings.EqualFold(r.Method, m.Method) {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if m.RateLimit > 0 && !s.endpointLimit.allow(m.Path, m.RateLimit) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, endpointMaxBodyBytes))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "body too large"})
		return
	}

	switch m.Auth {
	case "token":
		got := r.Header.Get("X-Webhook-Token")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(m.Secret)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
	case "hmac":
		mac := hmac.New(sha256.New, []byte(m.Secret))
		mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		got := strings.TrimPrefix(strings.ToLower(r.Header.Get("X-Webhook-Signature")), "sha256=")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
			return
		}
	}

	if re.Proc == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "handler not found: src/" + m.Handler + ".endpoint.os, процедура Обработать(Запрос, Ответ)"})
		return
	}

	reqObj := &endpointRequest{method: r.Method, path: path, body: string(body), headers: r.Header, query: r.URL.Query()}
	respObj := &endpointResponse{code: http.StatusOK, headers: map[string]string{}}

	// Полное DSL-окружение (Справочники/Документы/Запрос-объект/HTTP/...) — как
	// у обработок; Запрос/Ответ биндятся на параметры процедуры позиционно.
	mc := runtime.NewMovementsCollector("endpoint", uuid.Nil)
	vars := s.buildDSLVars(r.Context(), mc)
	if _, runErr := s.interp.Call(re.Proc, nil, []any{reqObj, respObj}, vars); runErr != nil {
		// Текст DSL-ошибки наружу не отдаём (информация о внутренностях);
		// владелец смотрит лог сервера.
		fmt.Fprintf(os.Stderr, "endpoint %s: ошибка обработчика: %v\n", m.Name, runErr)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "handler error"})
		return
	}

	for k, v := range respObj.headers {
		w.Header().Set(k, v)
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(respObj.code)
	_, _ = w.Write([]byte(respObj.body))
}

// ── DSL-объекты Запрос / Ответ ───────────────────────────────────────────────

// endpointRequest — DSL-объект «Запрос»: Метод/Тело/Путь как поля,
// Заголовок(имя)/Параметр(имя) как методы.
type endpointRequest struct {
	method  string
	path    string
	body    string
	headers http.Header
	query   url.Values
}

func (q *endpointRequest) Get(name string) any {
	switch strings.ToLower(name) {
	case "метод", "method":
		return q.method
	case "тело", "body":
		return q.body
	case "путь", "path":
		return q.path
	}
	return nil
}

func (q *endpointRequest) Set(string, any) {} // Запрос неизменяем из DSL

func (q *endpointRequest) CallMethod(method string, args []any) any {
	arg := ""
	if len(args) > 0 {
		arg = fmt.Sprintf("%v", args[0])
	}
	switch method {
	case "заголовок", "header":
		return q.headers.Get(arg)
	case "параметр", "param":
		return q.query.Get(arg)
	}
	return nil
}

// endpointResponse — DSL-объект «Ответ»: Код/Тело как поля (записываемые),
// УстановитьКод/УстановитьЗаголовок как методы.
type endpointResponse struct {
	code    int
	body    string
	headers map[string]string
}

func (p *endpointResponse) Get(name string) any {
	switch strings.ToLower(name) {
	case "код", "code":
		return p.code
	case "тело", "body":
		return p.body
	}
	return nil
}

func (p *endpointResponse) Set(name string, v any) {
	switch strings.ToLower(name) {
	case "код", "code":
		if f, ok := toEndpointInt(v); ok {
			p.code = f
		}
	case "тело", "body":
		p.body = fmt.Sprintf("%v", v)
	}
}

func (p *endpointResponse) CallMethod(method string, args []any) any {
	switch method {
	case "установитькод", "setcode", "setstatus":
		if len(args) > 0 {
			if n, ok := toEndpointInt(args[0]); ok {
				p.code = n
			}
		}
	case "установитьзаголовок", "setheader":
		if len(args) >= 2 {
			p.headers[fmt.Sprintf("%v", args[0])] = fmt.Sprintf("%v", args[1])
		}
	}
	return nil
}

// toEndpointInt приводит DSL-число (decimal/float/int) к int.
func toEndpointInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	}
	// decimal.Decimal и прочее — через строку
	if s := fmt.Sprintf("%v", v); s != "" {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
			return n, true
		}
	}
	return 0, false
}
