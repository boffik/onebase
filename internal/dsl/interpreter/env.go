package interpreter

import "strings"

// This is implemented by runtime.Object; defined here to avoid import cycles.
type This interface {
	Get(name string) any
	Set(name string, v any)
}

// MethodCallable is implemented by objects that support obj.Method(args) calls.
type MethodCallable interface {
	CallMethod(method string, args []any) any
}

// MapThis wraps map[string]any as a This (used for tablepart rows and register movement records).
type MapThis struct{ M map[string]any }

func (m *MapThis) Get(name string) any {
	low := strings.ToLower(name)
	for k, v := range m.M {
		if strings.ToLower(k) == low {
			return v
		}
	}
	return nil
}

func (m *MapThis) Set(name string, v any) {
	low := strings.ToLower(name)
	for k := range m.M {
		if strings.ToLower(k) == low {
			m.M[k] = v
			return
		}
	}
	m.M[low] = v
}

// execCtx — изменяемое состояние одного запуска DSL (Run/Call/RunWithResult/
// EvalExpr). Живёт в цепочке env конкретного вызова и разделяется всеми его
// кадрами, поэтому конкурентные запуски на одном *Interpreter не гонят по
// curFile/curLine и видят только свой debug hook (план 52).
type execCtx struct {
	curFile string // last executed statement location (for error reporting)
	curLine int
	debug   DebugHook // hook этого запуска; nil = без отладки, нулевые накладные
}

type env struct {
	vars   map[string]any
	parent *env
	this   This
	ec     *execCtx
	// depth — глубина вызова процедур/функций (корень = 1). Растёт на каждый
	// callUserProc; используется стражем рекурсии (см. limits.go). O(1) и
	// потокобезопасно: счётчик живёт в цепочке env конкретного запуска.
	depth int
}

func newEnv(this This) *env {
	return &env{vars: make(map[string]any), this: this, ec: &execCtx{}, depth: 1}
}

func (e *env) child() *env {
	return &env{vars: make(map[string]any), parent: e, this: e.this, ec: e.ec, depth: e.depth + 1}
}

func (e *env) get(name string) (any, bool) {
	low := strings.ToLower(name)
	if low == "this" || low == "этотобъект" {
		return e.this, true
	}
	name = low
	if v, ok := e.vars[name]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.get(name)
	}
	return nil, false
}

func (e *env) set(name string, v any) {
	name = strings.ToLower(name)
	// Если переменная уже объявлена в родительском scope — обновляем там.
	if _, ok := e.vars[name]; !ok && e.parent != nil {
		if e.parent.has(name) {
			e.parent.set(name, v)
			return
		}
	}
	e.vars[name] = v
}

// publishTemp временно записывает значения прямо в e.vars и возвращает
// функцию, восстанавливающую прежнее состояние этих ключей. Используется
// для служебных имён (ОписаниеОшибки), которые должны быть видны только
// внутри блока, но не должны протекать наружу как пользовательские
// переменные.
func publishTemp(e *env, vals map[string]any) func() {
	type prev struct {
		v       any
		existed bool
	}
	saved := make(map[string]prev, len(vals))
	for k, v := range vals {
		k = strings.ToLower(k)
		old, ok := e.vars[k]
		saved[k] = prev{old, ok}
		e.vars[k] = v
	}
	return func() {
		for k, p := range saved {
			if p.existed {
				e.vars[k] = p.v
			} else {
				delete(e.vars, k)
			}
		}
	}
}

func (e *env) has(name string) bool {
	name = strings.ToLower(name)
	if _, ok := e.vars[name]; ok {
		return true
	}
	if e.parent != nil {
		return e.parent.has(name)
	}
	return false
}
