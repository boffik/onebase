package interpreter

import (
	"fmt"
	"strconv"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/token"
)

// dslStop — системная остановка (Error без Попытки, внутренние ошибки интерпретатора)
type dslStop struct{ err error }

// dslReturn — ранний выход через Возврат
type dslReturn struct{ val any }

// userError — пользовательская ошибка через Error(), перехватывается Попыткой
type userError struct{ Msg string }


type Interpreter struct {
	LookupProc func(name string) *ast.ProcedureDecl
}

func New() *Interpreter { return &Interpreter{} }

// RunWithResult executes a function procedure and captures its return value.
func (i *Interpreter) RunWithResult(proc *ast.ProcedureDecl, this This, result *any, extraVars ...map[string]any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch s := r.(type) {
			case dslStop:
				err = s.err
			case userError:
				err = &DSLError{Msg: s.Msg}
			case dslReturn:
				if result != nil {
					*result = s.val
				}
			default:
				panic(r)
			}
		}
	}()
	e := newEnv(this)
	for _, m := range extraVars {
		for k, v := range m {
			e.set(k, v)
		}
	}
	i.execBlock(proc.Body, e)
	return nil
}

// Run executes a procedure. Optional extra vars (e.g. {"Движения": collector}) are
// injected into the top-level environment.
func (i *Interpreter) Run(proc *ast.ProcedureDecl, this This, extraVars ...map[string]any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch s := r.(type) {
			case dslStop:
				err = s.err
			case userError:
				err = &DSLError{Msg: s.Msg}
			case dslReturn:
				// early return from procedure — not an error
			default:
				panic(r)
			}
		}
	}()
	e := newEnv(this)
	for _, m := range extraVars {
		for k, v := range m {
			e.set(k, v)
		}
	}
	i.execBlock(proc.Body, e)
	return nil
}

func (i *Interpreter) execBlock(stmts []ast.Stmt, e *env) {
	for _, s := range stmts {
		i.execStmt(s, e)
	}
}

func (i *Interpreter) execStmt(s ast.Stmt, e *env) {
	switch v := s.(type) {
	case *ast.IfStmt:
		cond := i.evalExpr(v.Cond, e)
		if truthy(cond) {
			i.execBlock(v.Then, e.child())
		} else if len(v.Else) > 0 {
			i.execBlock(v.Else, e.child())
		}
	case *ast.ForEachStmt:
		coll := i.evalExpr(v.Collection, e)
		switch items := coll.(type) {
		case []map[string]any:
			for _, row := range items {
				child := e.child()
				child.set(v.Var.Literal, &MapThis{M: row})
				i.execBlock(v.Body, child)
			}
		case []any:
			for _, item := range items {
				child := e.child()
				child.set(v.Var.Literal, item)
				i.execBlock(v.Body, child)
			}
		case *Array:
			for _, item := range items.Iterate() {
				child := e.child()
				child.set(v.Var.Literal, item)
				i.execBlock(v.Body, child)
			}
		case *Map:
			for idx, key := range items.keys {
				child := e.child()
				child.set(v.Var.Literal, &KeyValue{Key: key, Value: items.vals[idx]})
				i.execBlock(v.Body, child)
			}
		}
	case *ast.AssignStmt:
		val := i.evalExpr(v.Value, e)
		i.assign(v.Target, val, e)
	case *ast.ExprStmt:
		i.evalExpr(v.X, e)
	case *ast.VarDecl:
		e.set(v.Name.Literal, nil)
	case *ast.NumericForStmt:
		start := toFloatOr0(i.evalExpr(v.Start, e))
		end := toFloatOr0(i.evalExpr(v.End, e))
		for counter := start; counter <= end; counter++ {
			child := e.child()
			child.set(v.Var.Literal, counter)
			i.execBlock(v.Body, child)
		}
	case *ast.ReturnStmt:
		var val any
		if v.Value != nil {
			val = i.evalExpr(v.Value, e)
		}
		panic(dslReturn{val: val})
	case *ast.TryStmt:
		i.execTry(v, e)
	}
}

func (i *Interpreter) assign(target ast.Expr, val any, e *env) {
	switch t := target.(type) {
	case *ast.Ident:
		e.set(t.Tok.Literal, val)
	case *ast.MemberExpr:
		obj := i.evalExpr(t.Object, e)
		switch o := obj.(type) {
		case This:
			o.Set(t.Field.Literal, val)
		case *Struct:
			o.Set(t.Field.Literal, val)
		}
	case *ast.IndexExpr:
		obj := i.evalExpr(t.Object, e)
		idx := i.evalExpr(t.Index, e)
		switch o := obj.(type) {
		case *Array:
			o.SetIndex(int(toFloatOr0(idx)), val)
		case *Map:
			o.CallMethod("Вставить", []any{idx, val})
		}
	}
}

func (i *Interpreter) evalExpr(expr ast.Expr, e *env) any {
	switch v := expr.(type) {
	case *ast.StringLit:
		return v.Value
	case *ast.NumberLit:
		f, _ := strconv.ParseFloat(v.Value, 64)
		return f
	case *ast.BoolLit:
		return v.Value
	case *ast.Ident:
		val, _ := e.get(v.Tok.Literal)
		return val
	case *ast.MemberExpr:
		obj := i.evalExpr(v.Object, e)
		switch o := obj.(type) {
		case This:
			return o.Get(v.Field.Literal)
		case *Struct:
			return o.Get(v.Field.Literal)
		case *KeyValue:
			return o.Get(v.Field.Literal)
		}
		return nil
	case *ast.IndexExpr:
		obj := i.evalExpr(v.Object, e)
		idx := i.evalExpr(v.Index, e)
		switch o := obj.(type) {
		case *Array:
			return o.Index(int(toFloatOr0(idx)))
		case *Map:
			return o.CallMethod("Получить", []any{idx})
		}
		return nil
	case *ast.NewExpr:
		return i.evalNew(v, e)
	case *ast.UnaryExpr:
		return i.evalUnary(v, e)
	case *ast.BinaryExpr:
		return i.evalBinary(v, e)
	case *ast.CallExpr:
		return i.evalCall(v, e)
	}
	return nil
}

func (i *Interpreter) evalNew(n *ast.NewExpr, e *env) any {
	args := i.evalArgs(n.Args, e)
	switch n.TypeName.Literal {
	case "Массив", "Array":
		return &Array{}
	case "Соответствие", "Map":
		return &Map{}
	case "Структура", "Structure":
		return newStruct(args)
	}
	return nil
}

func (i *Interpreter) evalUnary(u *ast.UnaryExpr, e *env) any {
	val := i.evalExpr(u.Operand, e)
	switch u.Op.Type {
	case token.NOT:
		return !truthy(val)
	case token.MINUS:
		f, _ := toFloat(val)
		return -f
	}
	return nil
}

func (i *Interpreter) evalBinary(b *ast.BinaryExpr, e *env) any {
	// short-circuit для AND/OR
	if b.Op.Type == token.AND {
		l := i.evalExpr(b.Left, e)
		if !truthy(l) {
			return false
		}
		return truthy(i.evalExpr(b.Right, e))
	}
	if b.Op.Type == token.OR {
		l := i.evalExpr(b.Left, e)
		if truthy(l) {
			return true
		}
		return truthy(i.evalExpr(b.Right, e))
	}
	l := i.evalExpr(b.Left, e)
	r := i.evalExpr(b.Right, e)
	switch b.Op.Type {
	case token.ASSIGN: // equality in conditions
		return equal(l, r)
	case token.NEQ:
		return !equal(l, r)
	case token.LT:
		return compare(l, r) < 0
	case token.GT:
		return compare(l, r) > 0
	case token.LTE:
		return compare(l, r) <= 0
	case token.GTE:
		return compare(l, r) >= 0
	case token.PLUS:
		lf, lok := toFloat(l)
		rf, rok := toFloat(r)
		if lok && rok {
			return lf + rf
		}
		return fmt.Sprintf("%v", l) + fmt.Sprintf("%v", r)
	case token.MINUS:
		lf, lok := toFloat(l)
		rf, rok := toFloat(r)
		if lok && rok {
			return lf - rf
		}
	case token.STAR:
		lf, lok := toFloat(l)
		rf, rok := toFloat(r)
		if lok && rok {
			return lf * rf
		}
	case token.SLASH:
		lf, lok := toFloat(l)
		rf, rok := toFloat(r)
		if lok && rok && rf != 0 {
			return lf / rf
		}
	}
	return nil
}

func (i *Interpreter) evalCall(c *ast.CallExpr, e *env) any {
	args := i.evalArgs(c.Args, e)
	switch callee := c.Callee.(type) {
	case *ast.Ident:
		fnName := callee.Tok.Literal
		if val, ok := e.get(fnName); ok {
			if bf, ok2 := val.(BuiltinFunc); ok2 {
				result, err := bf(args, callee.Tok.File, callee.Tok.Line)
				if err != nil {
					panic(dslStop{err: err})
				}
				return result
			}
		}
		if i.LookupProc != nil {
			if proc := i.LookupProc(fnName); proc != nil {
				return i.callUserProc(proc, e, args)
			}
		}
		fn, ok := builtins[fnName]
		if !ok {
			panic(dslStop{err: fmt.Errorf("%s:%d: unknown function %q", callee.Tok.File, callee.Tok.Line, fnName)})
		}
		result, err := fn(args, callee.Tok.File, callee.Tok.Line)
		if err != nil {
			panic(dslStop{err: err})
		}
		return result
	case *ast.MemberExpr:
		recv := i.evalExpr(callee.Object, e)
		switch o := recv.(type) {
		case MethodCallable:
			return o.CallMethod(callee.Field.Literal, args)
		case *Struct:
			return o.CallMethod(callee.Field.Literal, args)
		}
		return nil
	}
	return nil
}

func (i *Interpreter) callUserProc(proc *ast.ProcedureDecl, callEnv *env, args []any) (retVal any) {
	defer func() {
		if r := recover(); r != nil {
			switch s := r.(type) {
			case dslReturn:
				retVal = s.val
			default:
				panic(r)
			}
		}
	}()
	child := &env{vars: make(map[string]any), parent: callEnv, this: callEnv.this}
	for idx, param := range proc.Params {
		if idx < len(args) {
			child.set(param.Literal, args[idx])
		} else {
			child.set(param.Literal, nil)
		}
	}
	i.execBlock(proc.Body, child)
	return nil
}

func (i *Interpreter) evalArgs(exprs []ast.Expr, e *env) []any {
	args := make([]any, len(exprs))
	for idx, a := range exprs {
		args[idx] = i.evalExpr(a, e)
	}
	return args
}

func truthy(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t != ""
	}
	return true
}

func equal(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compare(a, b any) int {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		if af < bf {
			return -1
		}
		if af > bf {
			return 1
		}
		return 0
	}
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}

func toFloatOr0(v any) float64 {
	f, _ := toFloat(v)
	return f
}

// execTry выполняет Попытка/Исключение.
// Только userError перехватывается; системные паники и dslReturn пробрасываются дальше.
func (i *Interpreter) execTry(t *ast.TryStmt, e *env) {
	var caught *userError
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ue, ok := r.(userError); ok {
					caught = &ue
					return
				}
				panic(r) // dslReturn, dslStop, Go panic — пробрасываем
			}
		}()
		i.execBlock(t.Try, e)
	}()
	if caught != nil {
		if len(t.Except) == 0 {
			// Нет блока Исключение — пробрасываем ошибку дальше
			panic(*caught)
		}
		msg := caught.Msg
		exceptEnv := e.child()
		descFn := BuiltinFunc(func(args []any, file string, line int) (any, error) {
			return msg, nil
		})
		exceptEnv.set("ОписаниеОшибки", descFn)
		exceptEnv.set("ErrorDescription", descFn)
		i.execBlock(t.Except, exceptEnv)
	}
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
