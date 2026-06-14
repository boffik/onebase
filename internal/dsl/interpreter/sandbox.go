package interpreter

import (
	"errors"
	"time"

	"github.com/ivantit66/onebase/internal/dsl/ast"
)

var (
	errSandboxTimeout = errors.New("превышено максимальное время выполнения (песочница)")
	errSandboxIters   = errors.New("превышен лимит итераций цикла (песочница)")
)

// SandboxProfile описывает, что разрешено одному запуску DSL. Нулевое значение =
// «всё разрешено» = поведение по умолчанию (без регрессии).
type SandboxProfile struct {
	AllowNet     bool          // сеть: HTTP-клиент и email
	AllowFile    bool          // файловые builtins
	MaxWallClock time.Duration // 0 = без лимита времени
	MaxLoopIters int           // 0 = дефолт (maxWhileIter)
}

// RestrictedProfile — строгий профиль для недоверенного кода (ИИ/marketplace).
func RestrictedProfile() SandboxProfile {
	return SandboxProfile{
		AllowNet:     false,
		AllowFile:    false,
		MaxWallClock: 10 * time.Second,
		MaxLoopIters: 1_000_000,
	}
}

// Vars возвращает extraVars, навязывающие запреты возможностей профиля
// (сеть/email/файлы). Мержить ПОСЛЕ обычных переменных запуска, чтобы deny-
// guard'ы перекрыли стандартные функции. Разрешённые возможности не внедряются —
// остаются обычные функции (с глобальным предохранителем сети, план 62).
func (p SandboxProfile) Vars() map[string]any {
	m := map[string]any{}
	if !p.AllowNet {
		deny := NetGuard(func() error {
			return errors.New("сеть запрещена в этом режиме (песочница)")
		})
		for k, v := range NewHTTPFunctions(deny) {
			m[k] = v
		}
		for k, v := range NewEmailFunctions(nil, deny) {
			m[k] = v
		}
	}
	if !p.AllowFile {
		deny := FileGuard(func() error {
			return errors.New("файловые операции запрещены в этом режиме (песочница)")
		})
		for k, v := range NewFileFunctions(deny) {
			m[k] = v
		}
	}
	return m
}

// RunSandboxed исполняет процедуру с ресурсными лимитами профиля (wall-clock и
// итерации). Запреты возможностей (сеть/файлы) подаются вызывающим через
// extraVars (см. SandboxProfile.Vars). Возвращаемое значение — в result.
func (i *Interpreter) RunSandboxed(proc *ast.ProcedureDecl, this This, p SandboxProfile, result *any, extraVars ...map[string]any) (err error) {
	e := i.startEnv(this)
	if p.MaxWallClock > 0 {
		e.ec.deadline = time.Now().Add(p.MaxWallClock)
	}
	e.ec.maxLoopIters = p.MaxLoopIters
	defer func() {
		if r := recover(); r != nil {
			switch s := r.(type) {
			case dslStop:
				err = s.err
			case userError:
				err = &DSLError{File: e.ec.curFile, Line: e.ec.curLine, Msg: s.Msg, Err: s.Err}
			case dslReturn:
				if result != nil {
					*result = s.val
				}
			default:
				panic(r)
			}
		}
	}()
	for _, m := range extraVars {
		for k, v := range m {
			e.set(k, v)
		}
	}
	i.execBlock(proc.Body, e)
	return nil
}
