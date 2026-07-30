package interpreter

import (
	"fmt"
	"strings"
)

// DSL-объект Утверждать (план 108) — встроенный набор ассертов для тестов
// уровня конфигурации. Единый модуль ассертов, чтобы каждый проект не
// переизобретал свой (как самодельные Проверить/РавныЛи). Инжектируется
// раннером тестов (`onebase test`) в прогон тест-обработки как переменная
// «Утверждать»:
//
//	Утверждать.Равно(НормализоватьТелефон("8 999 …"), "7999…", "8-формат → 7");
//	Утверждать.Истина(Условие, "описание");
//	Утверждать.Заполнено(Значение, "описание");
//
// Семантика — soft-assert: провал проверки НЕ прерывает тест-обработку, а
// помечает её проваленной и продолжает (чтобы одна обработка отчиталась сразу
// по нескольким проверкам). Каждый метод возвращает Булево (прошла ли
// проверка) — тест при желании может ветвиться по результату.

// AssertOutcome — результат одной проверки Утверждать.*.
type AssertOutcome struct {
	Passed bool
	Desc   string // описание проверки (последний строковый аргумент)
	Detail string // деталь расхождения для отчёта (пусто, если прошла)
}

// AssertRecorder принимает результаты проверок из объекта Утверждать.
// Реализуется раннером тестов.
type AssertRecorder interface {
	RecordAssert(o AssertOutcome)
}

// AssertRoot — корневой DSL-объект Утверждать.
type AssertRoot struct{ rec AssertRecorder }

// NewAssertRoot создаёт объект для инжекции как DSL-переменную «Утверждать».
func NewAssertRoot(rec AssertRecorder) *AssertRoot { return &AssertRoot{rec: rec} }

// This: у объекта нет доступных членов, только методы. Get/Set — безопасные no-op.
func (a *AssertRoot) Get(string) any  { return nil }
func (a *AssertRoot) Set(string, any) {}

func (a *AssertRoot) CallMethod(method string, args []any) any {
	switch strings.ToLower(method) {
	case "равно", "equal":
		return a.equalAssert(args, true)
	case "неравно", "notequal":
		return a.equalAssert(args, false)
	case "истина", "true":
		return a.boolAssert(args, true)
	case "ложь", "false":
		return a.boolAssert(args, false)
	case "заполнено", "filled":
		return a.filledAssert(args)
	case "провалить", "fail":
		return a.failAssert(args)
	}
	panic(userError{Msg: "Утверждать: неизвестный метод «" + method +
		"» (доступны Равно, НеРавно, Истина, Ложь, Заполнено, Провалить)"})
}

// Равно(Факт, Ожидание, Описание) / НеРавно(Факт, Ожидание, Описание).
func (a *AssertRoot) equalAssert(args []any, wantEqual bool) any {
	fact := argAt(args, 0)
	expect := argAt(args, 1)
	desc := descAt(args, 2)
	passed := equal(fact, expect) == wantEqual
	detail := ""
	if !passed {
		if wantEqual {
			detail = fmt.Sprintf("ожидалось «%s», получено «%s»", assertStr(expect), assertStr(fact))
		} else {
			detail = fmt.Sprintf("ожидалось не равно «%s», а получено именно оно", assertStr(expect))
		}
	}
	return a.record(passed, desc, detail)
}

// Истина(Условие, Описание) / Ложь(Условие, Описание).
func (a *AssertRoot) boolAssert(args []any, want bool) any {
	cond := truthy(argAt(args, 0))
	desc := descAt(args, 1)
	passed := cond == want
	detail := ""
	if !passed {
		detail = fmt.Sprintf("ожидалось %v, получено %v", want, cond)
	}
	return a.record(passed, desc, detail)
}

// Заполнено(Значение, Описание) — значение не пустое (та же семантика, что и
// ЗначениеЗаполнено).
func (a *AssertRoot) filledAssert(args []any) any {
	passed := !isBlankVal(argAt(args, 0))
	desc := descAt(args, 1)
	detail := ""
	if !passed {
		detail = "значение не заполнено"
	}
	return a.record(passed, desc, detail)
}

// Провалить(Описание) — безусловный провал (для недостижимых веток).
func (a *AssertRoot) failAssert(args []any) any {
	return a.record(false, descAt(args, 0), "явный Провалить")
}

func (a *AssertRoot) record(passed bool, desc, detail string) any {
	if a.rec != nil {
		a.rec.RecordAssert(AssertOutcome{Passed: passed, Desc: desc, Detail: detail})
	}
	return passed
}

func argAt(args []any, i int) any {
	if i < len(args) {
		return args[i]
	}
	return nil
}

func descAt(args []any, i int) string {
	if i < len(args) {
		return assertStr(args[i])
	}
	return ""
}

func assertStr(v any) string {
	if s, err := builtinToString([]any{v}, "", 0); err == nil {
		if str, ok := s.(string); ok {
			return str
		}
	}
	return fmt.Sprintf("%v", v)
}
