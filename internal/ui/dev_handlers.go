package ui

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/ivantit66/onebase/internal/query"
	"github.com/ivantit66/onebase/internal/runtime"
)

// ─── Query Console ──────────────────────────────────────────────────────────

func (s *Server) queryConsolePage(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	sources := s.buildQuerySources()
	schemaJSON, _ := json.Marshal(sources)
	s.render(w, r, "page-query-console", map[string]any{
		"Schema": template.JS(schemaJSON),
	})
}

func (s *Server) queryConsoleExec(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}

	var req struct {
		Query  string                 `json:"query"`
		Params map[string]any         `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResp(w, 400, map[string]any{"error": "Некорректный запрос"})
		return
	}

	res, err := query.Compile(req.Query, query.CompileOpts{
		Params:      req.Params,
		Registers:   s.reg.Registers(),
		InfoRegs:    s.reg.InfoRegisters(),
		AccountRegs: s.reg.AccountRegisters(),
		Dialect:     s.store.Dialect(),
	})
	if err != nil {
		jsonResp(w, 200, map[string]any{"error": "Ошибка запроса: " + err.Error()})
		return
	}

	start := time.Now()
	rows, err := s.store.QueryAll(r.Context(), res.SQL, res.Args...)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		jsonResp(w, 200, map[string]any{"error": "Ошибка выполнения: " + err.Error()})
		return
	}

	columns := []string{}
	if len(rows) > 0 {
		for k := range rows[0] {
			columns = append(columns, k)
		}
	}

	dataRows := make([][]any, len(rows))
	for i, row := range rows {
		vals := make([]any, len(columns))
		for j, col := range columns {
			vals[j] = row[col]
		}
		dataRows[i] = vals
	}

	jsonResp(w, 200, map[string]any{
		"columns": columns,
		"rows":    dataRows,
		"count":   len(rows),
		"time":    elapsed.String(),
	})
}

// ─── Code Console ───────────────────────────────────────────────────────────

func (s *Server) codeConsolePage(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	s.render(w, r, "page-code-console", nil)
}

func (s *Server) codeConsoleExec(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResp(w, 400, map[string]any{"error": "Некорректный запрос"})
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		jsonResp(w, 200, map[string]any{"error": "Пустой код"})
		return
	}

	// Wrap in procedure if user didn't provide one
	lower := strings.ToLower(code)
	if !strings.HasPrefix(lower, "процедура") && !strings.HasPrefix(lower, "procedure") &&
		!strings.HasPrefix(lower, "функция") && !strings.HasPrefix(lower, "function") {
		code = "Процедура __Console()\n" + code + "\nКонецПроцедуры"
	}

	l := lexer.New(code, "<console>")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	if err != nil {
		jsonResp(w, 200, map[string]any{"error": "Ошибка синтаксиса: " + err.Error()})
		return
	}
	if len(prog.Procedures) == 0 {
		jsonResp(w, 200, map[string]any{"error": "Нет процедур для выполнения"})
		return
	}

	mc := runtime.NewMovementsCollector("console", uuid.Nil)
	var msgs []string
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	vars := s.buildDSLVarsWithMessages(ctx, mc, &msgs)
	proc := prog.Procedures[0]

	runErr := ""
	if err := s.interp.Run(proc, nil, vars); err != nil {
		runErr = err.Error()
	}

	resp := map[string]any{
		"output": msgs,
	}
	if runErr != "" {
		resp["error"] = runErr
	}

	jsonResp(w, 200, resp)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func jsonResp(w http.ResponseWriter, status int, data map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
