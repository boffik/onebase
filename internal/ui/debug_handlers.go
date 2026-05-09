package ui

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/debugger"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
)

// debugCtrl is the global debug controller, initialized lazily
var debugCtrl *debugger.DebugController

func getDebugController() *debugger.DebugController {
	if debugCtrl == nil {
		debugCtrl = debugger.NewDebugController()
	}
	return debugCtrl
}

// ── JSON helpers ─────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("empty body")
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// ── Handlers ─────────────────────────────────────────────────────

// debugConsole renders the debug console page
func (s *Server) debugConsole(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "debug_console", map[string]any{
		"Title": "Консоль кода",
	})
}

// debugEvaluate handles POST /debug/evaluate — evaluate a DSL expression
func (s *Server) debugEvaluate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Expr     string `json:"expr"`
		Session  string `json:"session,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if req.Expr == "" {
		writeJSON(w, 400, map[string]string{"error": "empty expression"})
		return
	}

	// If there's an active paused session, evaluate in its context
	dc := getDebugController()
	if req.Session != "" {
		sess := dc.GetSession(req.Session)
		if sess != nil {
			result := sess.Evaluate(req.Expr, func(expr string) (any, error) {
				return standaloneEval(s, expr)
			})
			writeJSON(w, 200, result)
			return
		}
	}

	// Standalone evaluation (no debug session)
	val, err := standaloneEval(s, req.Expr)
	if err != nil {
		writeJSON(w, 200, debugger.EvaluateResult{
			IsError: true,
			Error:   err.Error(),
		})
		return
	}
	writeJSON(w, 200, debugger.EvaluateResult{
		Value: val,
		Type:  debugger.GetTypeName(val),
	})
}

// standaloneEval parses and evaluates a DSL expression using a temporary interpreter
func standaloneEval(s *Server, expr string) (any, error) {
	l := lexer.New(expr, "<console>")
	p := parser.New(l)
	parsed, err := p.ParseExpr()
	if err != nil {
		return nil, err
	}

	tmpInterp := interpreter.New()
	tmpInterp.LookupProc = s.reg.GetModuleProc
	result := tmpInterp.EvalExpr(parsed, nil)
	return result, nil
}

// debugStart handles POST /debug/start — begin debugging a module
func (s *Server) debugStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Module string `json:"module"`
		File   string `json:"file"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	dc := getDebugController()
	sess := dc.StartSession(req.File)

	// Launch interpreter in a goroutine
	go func() {
		// Create a debug interpreter
		dbgInterp := interpreter.New()
		dbgInterp.LookupProc = s.reg.GetModuleProc
		dbgInterp.DebugHook = sess // ActiveSession implements DebugHook

		// Look up the procedure
		proc := s.reg.GetModuleProc(req.Module)
		if proc == nil {
			sess.Stop()
			return
		}

		// Run the procedure
		dbgInterp.Run(proc, nil)
		sess.Stop()
	}()

	writeJSON(w, 200, map[string]string{
		"session_id": sess.ID,
		"state":      "running",
	})
}

// debugStop handles POST /debug/stop
func (s *Server) debugStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Session string `json:"session"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	dc := getDebugController()
	dc.RemoveSession(req.Session)
	writeJSON(w, 200, map[string]string{"status": "stopped"})
}

// debugStatus handles GET /debug/status — polling for debug state
func (s *Server) debugStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		writeJSON(w, 200, map[string]string{"state": "idle"})
		return
	}

	dc := getDebugController()
	sess := dc.GetSession(sessionID)
	if sess == nil {
		writeJSON(w, 200, map[string]string{"state": "idle"})
		return
	}

	writeJSON(w, 200, sess.Snapshot())
}

// debugSetBreakpoint handles POST /debug/breakpoint
func (s *Server) debugSetBreakpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Session   string `json:"session"`
		File      string `json:"file"`
		Line      int    `json:"line"`
		Condition string `json:"condition,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	dc := getDebugController()
	sess := dc.GetSession(req.Session)
	if sess == nil {
		writeJSON(w, 404, map[string]string{"error": "session not found"})
		return
	}

	bp := sess.SetBreakpoint(req.File, req.Line, req.Condition)
	writeJSON(w, 200, bp)
}

// debugRemoveBreakpoint handles DELETE /debug/breakpoint/{file}/{line}
func (s *Server) debugRemoveBreakpoint(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	file := chi.URLParam(r, "file")
	lineStr := chi.URLParam(r, "line")
	if sessionID == "" || file == "" || lineStr == "" {
		writeJSON(w, 400, map[string]string{"error": "missing params"})
		return
	}

	var line int
	fmt.Sscanf(lineStr, "%d", &line)

	dc := getDebugController()
	sess := dc.GetSession(sessionID)
	if sess == nil {
		writeJSON(w, 404, map[string]string{"error": "session not found"})
		return
	}

	sess.RemoveBreakpoint(file, line)
	writeJSON(w, 200, map[string]string{"status": "removed"})
}

// debugContinue handles POST /debug/continue
func (s *Server) debugContinue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Session string `json:"session"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	dc := getDebugController()
	sess := dc.GetSession(req.Session)
	if sess == nil {
		writeJSON(w, 404, map[string]string{"error": "session not found"})
		return
	}

	sess.Continue()
	writeJSON(w, 200, map[string]string{"status": "continued"})
}

// debugStep handles POST /debug/step
func (s *Server) debugStep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Session string `json:"session"`
		Mode    string `json:"mode"` // "into", "over", "out"
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	dc := getDebugController()
	sess := dc.GetSession(req.Session)
	if sess == nil {
		writeJSON(w, 404, map[string]string{"error": "session not found"})
		return
	}

	var mode debugger.StepMode
	switch req.Mode {
	case "into":
		mode = debugger.StepInto
	case "over":
		mode = debugger.StepOver
	case "out":
		mode = debugger.StepOut
	default:
		mode = debugger.StepOver
	}

	sess.Step(mode)
	writeJSON(w, 200, map[string]string{"status": "stepped"})
}
