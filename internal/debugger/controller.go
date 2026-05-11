package debugger

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// DebugController manages all debug sessions. Thread-safe.
type DebugController struct {
	mu       sync.Mutex
	sessions map[string]*ActiveSession
}

// NewDebugController creates a new controller
func NewDebugController() *DebugController {
	return &DebugController{
		sessions: make(map[string]*ActiveSession),
	}
}

// ActiveSession is a live debug session with channel-based pause/resume.
// The interpreter goroutine blocks inside Pause(); the HTTP handler unblocks
// it via Continue() or Step().
type ActiveSession struct {
	mu       sync.Mutex
	ID       string
	ModulePath string
	State    DebugState

	// Breakpoints indexed by file -> line -> bp
	breakpoints map[string]map[int]*Breakpoint

	// Snapshot captured at pause time
	currentLoc *Location
	callStack  []StackFrame
	vars       map[string]any

	// Diagnostics — last check info
	diagLastFile string
	diagLastLine int
	diagMessages []string

	// Stepping
	stepMode  StepMode
	stepDepth int
	lastDepth int // interpreter's actual stack depth from last HookShouldStep call

	// Channels: interpreter goroutine uses pauseChan/resumeChan,
	// HTTP handlers signal via methods.
	pauseChan  chan struct{} // signaled when interpreter pauses
	resumeChan chan struct{} // signaled by HTTP handler to resume
	doneChan   chan struct{} // closed when session ends
	stopOnce   sync.Once    // ensures doneChan is closed only once

	// Expression evaluation during pause
	evalReq chan evalRequest
}

type evalRequest struct {
	expr   string
	result chan EvaluateResult
}

// StartSession creates and registers a new debug session
func (dc *DebugController) StartSession(modulePath string) *ActiveSession {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	id := uuid.New().String()[:8]
	s := &ActiveSession{
		ID:          id,
		ModulePath:  modulePath,
		State:       StateRunning,
		breakpoints: make(map[string]map[int]*Breakpoint),
		vars:        make(map[string]any),
		pauseChan:   make(chan struct{}, 1),
		resumeChan:  make(chan struct{}),
		doneChan:    make(chan struct{}),
		evalReq:     make(chan evalRequest),
	}
	dc.sessions[id] = s
	return s
}

// GetSession returns a session by ID
func (dc *DebugController) GetSession(id string) *ActiveSession {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.sessions[id]
}

// RemoveSession removes and stops a session
func (dc *DebugController) RemoveSession(id string) {
	dc.mu.Lock()
	s := dc.sessions[id]
	if s != nil {
		delete(dc.sessions, id)
	}
	dc.mu.Unlock()

	if s != nil {
		s.Stop()
	}
}

// ── Breakpoint management ────────────────────────────────────────

// SetBreakpoint creates or updates a breakpoint
func (s *ActiveSession) SetBreakpoint(file string, line int, condition string) *Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.breakpoints[file] == nil {
		s.breakpoints[file] = make(map[int]*Breakpoint)
	}
	if bp, ok := s.breakpoints[file][line]; ok {
		bp.Condition = condition
		return bp
	}
	bp := &Breakpoint{
		ID:        fmt.Sprintf("bp-%d-%s", line, s.ID),
		File:      file,
		Line:      line,
		Enabled:   true,
		Condition: condition,
		CreatedAt: time.Now(),
	}
	s.breakpoints[file][line] = bp
	bp.MapLen = len(s.breakpoints)       // diagnostic: map length after store
	bp.EntryLen = len(s.breakpoints[file]) // diagnostic: entries for this file
	return bp
}

// RemoveBreakpoint deletes a breakpoint
func (s *ActiveSession) RemoveBreakpoint(file string, line int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if locMap, ok := s.breakpoints[file]; ok {
		if _, ok := locMap[line]; ok {
			delete(locMap, line)
			return true
		}
	}
	return false
}

// ToggleBreakpoint enables or disables a breakpoint at file:line
func (s *ActiveSession) ToggleBreakpoint(file string, line int) *Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	if locMap, ok := s.breakpoints[file]; ok {
		if bp, ok := locMap[line]; ok {
			bp.Enabled = !bp.Enabled
			return bp
		}
	}
	return nil
}

// CheckBreakpoint returns the breakpoint if there's an enabled one at file:line.
// The file parameter is normalized before lookup to match the editor ID format
// used by the configurator (e.g., "post-ПоступлениеТоваров").
func (s *ActiveSession) CheckBreakpoint(file string, line int) *Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := normalizeFilePath(file)
	s.diagLastFile = file
	s.diagLastLine = line
	s.diagMessages = append(s.diagMessages, fmt.Sprintf("check raw=%q line=%d norm=%q", file, line, normalized))
	// Keep last 50 messages
	if len(s.diagMessages) > 50 {
		s.diagMessages = s.diagMessages[len(s.diagMessages)-50:]
	}
	for key, locMap := range s.breakpoints {
		keyNorm := normalizeFilePath(key)
		match := strings.EqualFold(keyNorm, normalized)
		s.diagMessages = append(s.diagMessages, fmt.Sprintf("  bp key=%q keyNorm=%q match=%v", key, keyNorm, match))
		if match {
			for bpLine, bp := range locMap {
				s.diagMessages = append(s.diagMessages, fmt.Sprintf("  line cmp: bpLine=%d curLine=%d", bpLine, line))
				if bpLine == line && bp.Enabled {
					bp.HitCount++
					return bp
				}
			}
		}
	}
	return nil
}

// normalizeFilePath converts a file path or editor ID to a canonical form
// for case-insensitive breakpoint matching.
// Preserves original casing of the entity name so UI can match editor IDs.
func normalizeFilePath(file string) string {
	base := filepath.Base(file)
	baseLow := strings.ToLower(base)

	if strings.HasSuffix(baseLow, ".posting.os") {
		name := base[:len(base)-len(".posting.os")]
		return "post-" + capitalizeFirst(name)
	}
	if strings.HasSuffix(baseLow, ".module.os") {
		name := base[:len(base)-len(".module.os")]
		return "mod-" + capitalizeFirst(name)
	}
	if strings.HasSuffix(baseLow, ".proc.os") {
		name := base[:len(base)-len(".proc.os")]
		return "proc-" + capitalizeFirst(name)
	}
	if strings.HasSuffix(baseLow, ".os") {
		name := base[:len(base)-len(".os")]
		return capitalizeFirst(name)
	}
	// Already an editor ID like "post-ПоступлениеТоваров" — keep as-is
	return file
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// GetBreakpoints returns all breakpoints
func (s *ActiveSession) GetBreakpoints() []*Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []*Breakpoint
	for _, locMap := range s.breakpoints {
		for _, bp := range locMap {
			result = append(result, bp)
		}
	}
	return result
}

// GetBreakpointsForFile returns breakpoints for a file
func (s *ActiveSession) GetBreakpointsForFile(file string) []*Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	locMap := s.breakpoints[file]
	result := make([]*Breakpoint, 0, len(locMap))
	for _, bp := range locMap {
		result = append(result, bp)
	}
	return result
}

// HasBreakpointsForFile checks if there are any breakpoints for a file
func (s *ActiveSession) HasBreakpointsForFile(file string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.breakpoints[file]) > 0
}

// ── Call stack ───────────────────────────────────────────────────

// PushFrame pushes a call stack frame (implements DebugHook)
func (s *ActiveSession) PushFrame(procedure string, line int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callStack = append(s.callStack, StackFrame{Procedure: procedure, Line: line})
}

// PopFrame pops a call stack frame
func (s *ActiveSession) PopFrame() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.callStack) > 0 {
		s.callStack = s.callStack[:len(s.callStack)-1]
	}
}

// GetCallStack returns the current call stack
func (s *ActiveSession) GetCallStack() []StackFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]StackFrame, len(s.callStack))
	copy(result, s.callStack)
	return result
}

// StackDepth returns current call stack depth
func (s *ActiveSession) StackDepth() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.callStack)
}

// ── Pause / Resume / Step ────────────────────────────────────────

// Pause is called by the interpreter goroutine when it hits a breakpoint or step.
// It blocks until Continue/Step/Stop is called from an HTTP handler.
// The evalFn callback is called for expression evaluation requests during pause.
func (s *ActiveSession) Pause(loc Location, vars map[string]any, stack []StackFrame, evalFn func(string) (any, error)) {
	s.mu.Lock()
	s.State = StatePaused
	s.currentLoc = &loc
	s.vars = vars
	s.callStack = stack
	s.mu.Unlock()

	// Signal that we're paused (non-blocking write to buffered channel)
	select {
	case s.pauseChan <- struct{}{}:
	default:
	}

	// Block until resume/stop or evaluate request
	for {
		select {
		case <-s.resumeChan:
			return
		case <-s.doneChan:
			return
		case req := <-s.evalReq:
			val, err := evalFn(req.expr)
			if err != nil {
				req.result <- EvaluateResult{IsError: true, Error: err.Error()}
			} else {
				req.result <- EvaluateResult{Value: val, Type: GetTypeName(val)}
			}
		}
	}
}

// PauseChan returns the channel that is signaled when the interpreter pauses.
// Used by HTTP handler to wait for pause events.
func (s *ActiveSession) PauseChan() <-chan struct{} {
	return s.pauseChan
}

// Continue unblocks the interpreter goroutine (called from HTTP handler)
func (s *ActiveSession) Continue() {
	s.mu.Lock()
	s.State = StateRunning
	s.stepMode = StepNone
	ch := s.resumeChan
	s.mu.Unlock()

	select {
	case ch <- struct{}{}:
	default:
	}
}

// Step sets stepping mode and resumes
func (s *ActiveSession) Step(mode StepMode) {
	s.mu.Lock()
	s.State = StateRunning
	s.stepMode = mode
	s.stepDepth = s.lastDepth // use interpreter's actual depth from last pause
	ch := s.resumeChan
	s.mu.Unlock()

	select {
	case ch <- struct{}{}:
	default:
	}
}

// Stop terminates the session. Safe to call multiple times.
func (s *ActiveSession) Stop() {
	s.mu.Lock()
	s.State = StateStopped
	s.mu.Unlock()

	s.stopOnce.Do(func() { close(s.doneChan) })
}

// ShouldStep checks if execution should pause for stepping at the current position.
// currentDepth is the interpreter's actual call stack depth (from env parent chain).
func (s *ActiveSession) ShouldStep(currentDepth int) bool {
	s.mu.Lock()
	// Always store the interpreter's depth so Step() can use it for stepDepth
	s.lastDepth = currentDepth
	mode := s.stepMode
	sd := s.stepDepth
	s.mu.Unlock()

	if mode == StepNone {
		return false
	}
	switch mode {
	case StepOver:
		return currentDepth <= sd
	case StepInto:
		return true
	case StepOut:
		return currentDepth < sd
	}
	return false
}

// ── State queries (for HTTP API) ─────────────────────────────────

// Snapshot returns the current debug state for the status polling API
func (s *ActiveSession) Snapshot() StatusSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := StatusSnapshot{
		State:        s.State,
		Location:     s.currentLoc,
		Stack:        make([]StackFrame, len(s.callStack)),
		Breakpoints:  make([]Breakpoint, 0),
		DiagLastFile: s.diagLastFile,
		DiagLastLine: s.diagLastLine,
		DiagMessages: s.diagMessages,
	}
	// Collect breakpoint keys for diagnostics
	for key, locMap := range s.breakpoints {
		snap.DiagBPKeys = append(snap.DiagBPKeys, key)
		snap.DiagBPCount += len(locMap)
	}
	copy(snap.Stack, s.callStack)

	for _, v := range s.vars {
		_ = v // skip internal vars
	}
	for name, val := range s.vars {
		if name == "__debug_session" {
			continue
		}
		snap.Variables = append(snap.Variables, VarEntry{
			Name:  name,
			Value: FormatValue(val),
			Type:  GetTypeName(val),
		})
	}

	for _, locMap := range s.breakpoints {
		for _, bp := range locMap {
			snap.Breakpoints = append(snap.Breakpoints, *bp)
		}
	}

	return snap
}

// Evaluate sends an expression to the paused interpreter and waits for the result.
// Called from HTTP handler. Times out after 5 seconds.
func (s *ActiveSession) Evaluate(expr string, evalFn func(string) (any, error)) EvaluateResult {
	req := evalRequest{
		expr:   expr,
		result: make(chan EvaluateResult, 1),
	}

	select {
	case s.evalReq <- req:
	case <-time.After(5 * time.Second):
		return EvaluateResult{IsError: true, Error: "evaluation timed out (not paused?)"}
	}

	select {
	case r := <-req.result:
		return r
	case <-time.After(5 * time.Second):
		return EvaluateResult{IsError: true, Error: "evaluation timed out"}
	}
}

// ── DebugHook interface implementation ───────────────────────────
// These methods satisfy interpreter.DebugHook interface.
// Named HookXxx to avoid collision with ActiveSession's own methods.

func (s *ActiveSession) HookCheckBreakpoint(file string, line int) bool {
	return s.CheckBreakpoint(file, line) != nil
}

func (s *ActiveSession) HookShouldStep(depth int) bool {
	return s.ShouldStep(depth)
}

func (s *ActiveSession) HookOnPause(file string, line int, vars map[string]any, evalFn func(string) (any, error)) {
	loc := Location{File: normalizeFilePath(file), Line: line}
	stack := s.GetCallStack()
	s.Pause(loc, vars, stack, evalFn)
}

func (s *ActiveSession) HookPushFrame(procedure string, line int) {
	s.PushFrame(procedure, line)
}

func (s *ActiveSession) HookPopFrame() {
	s.PopFrame()
}

// ── GlobalDebugController ─────────────────────────────────────────

// GlobalDebugController manages a single global debug session used for
// debugging DSL modules across the entire application.
type GlobalDebugController struct {
	mu      sync.Mutex
	enabled bool
	session *ActiveSession
}

// NewGlobalDebugController creates a new global debug controller (disabled by default).
func NewGlobalDebugController() *GlobalDebugController {
	return &GlobalDebugController{}
}

// Enable creates a new ActiveSession with ID "global" and ModulePath "*",
// then marks the controller as enabled. If there was an existing session
// it is stopped first.
func (g *GlobalDebugController) Enable() *ActiveSession {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.session != nil {
		g.session.Stop()
	}

	s := &ActiveSession{
		ID:          "global",
		ModulePath:  "*",
		State:       StateRunning,
		breakpoints: make(map[string]map[int]*Breakpoint),
		vars:        make(map[string]any),
		pauseChan:   make(chan struct{}, 1),
		resumeChan:  make(chan struct{}),
		doneChan:    make(chan struct{}),
		evalReq:     make(chan evalRequest),
	}
	g.session = s
	g.enabled = true
	return s
}

// Disable stops the current session (if any), clears it, and marks the controller as disabled.
func (g *GlobalDebugController) Disable() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.session != nil {
		g.session.Stop()
		g.session = nil
	}
	g.enabled = false
}

// Session returns the current active session, or nil if disabled.
func (g *GlobalDebugController) Session() *ActiveSession {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.session
}

// IsEnabled returns whether the global debug controller is enabled.
func (g *GlobalDebugController) IsEnabled() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.enabled
}

// SetSession sets the session directly (used when wiring up from ui.Server).
func (g *GlobalDebugController) SetSession(sess *ActiveSession) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.session != nil {
		g.session.Stop()
	}
	g.session = sess
	g.enabled = sess != nil
}
