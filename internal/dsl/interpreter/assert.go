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

// RoleChecker резолвит, разрешает ли именованная роль операцию над (вид, объект).
// Инжектится раннером тестов (слой ui), чтобы ядро интерпретатора не зависело от
// пакета auth. Вид/операция — пользовательские слова, реализация их нормализует.
// Ошибка (неизвестная роль/вид) — чтобы ассерт провалился громко, а не молча
// отчитался «не разрешено».
type RoleChecker interface {
	RoleAllows(roleName, kind, entity, op string) (allowed bool, err error)
}

// AssertRoot — корневой DSL-объект Утверждать.
type AssertRoot struct {
	rec   AssertRecorder
	roles RoleChecker // nil вне `onebase test` — ассерты ролей тогда проваливаются с пояснением
}

// NewAssertRoot создаёт объект для инжекции как DSL-переменную «Утверждать».
func NewAssertRoot(rec AssertRecorder) *AssertRoot { return &AssertRoot{rec: rec} }

// SetRoleChecker включает ассерты РольМожет/РольНеМожет, подставляя резолвер прав.
func (a *AssertRoot) SetRoleChecker(rc RoleChecker) { a.roles = rc }

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
	case "рольможет", "rolecan":
		return a.roleAssert(args, true)
	case "рольнеможет", "rolecannot":
		return a.roleAssert(args, false)
	}
	panic(userError{Msg: "Утверждать: неизвестный метод «" + method +
		"» (доступны Равно, НеРавно, Истина, Ложь, Заполнено, Провалить, РольМожет, РольНеМожет)"})
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

// РольМожет(Роль, Вид, Объект, Операция, Описание) /
// РольНеМожет(...) — проверка матрицы прав роли поверх настоящего движка
// (auth.PermissionHas). Вид: справочник|документ|регистр|регистрсведений|отчёт|
// обработка; Операция: read/write/post/unpost/delete/run и русские синонимы
// (провести, изменять, …). Источник ролей — roles/*.yaml проекта.
func (a *AssertRoot) roleAssert(args []any, want bool) any {
	role := assertStr(argAt(args, 0))
	kind := assertStr(argAt(args, 1))
	entity := assertStr(argAt(args, 2))
	op := assertStr(argAt(args, 3))
	desc := descAt(args, 4)
	if a.roles == nil {
		return a.record(false, desc, "проверка ролей доступна только в onebase test")
	}
	allowed, err := a.roles.RoleAllows(role, kind, entity, op)
	if err != nil {
		return a.record(false, desc, err.Error())
	}
	passed := allowed == want
	detail := ""
	if !passed {
		verb := "должна разрешать"
		if !want {
			verb = "не должна разрешать"
		}
		detail = fmt.Sprintf("роль «%s» %s %s «%s» операцию «%s»", role, verb, kind, entity, op)
	}
	return a.record(passed, desc, detail)
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
