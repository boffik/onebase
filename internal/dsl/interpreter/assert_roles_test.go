package interpreter

import (
	"errors"
	"testing"
)

type capRecorder struct{ outcomes []AssertOutcome }

func (c *capRecorder) RecordAssert(o AssertOutcome) { c.outcomes = append(c.outcomes, o) }

func (c *capRecorder) last() AssertOutcome { return c.outcomes[len(c.outcomes)-1] }

// fakeAccessChecker — управляемая заглушка interpreter.AccessChecker.
type fakeAccessChecker struct {
	allow     bool
	masked    any
	hasPolicy bool
	rowState  string
	err       error
}

func (f fakeAccessChecker) RoleAllows(_, _, _, _ string) (bool, error) { return f.allow, f.err }

func (f fakeAccessChecker) FieldMask(_, _, _, _ string, v any) (any, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	if f.masked != nil {
		return f.masked, f.hasPolicy, nil
	}
	return v, f.hasPolicy, nil
}

func (f fakeAccessChecker) RowRestriction(_, _, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.rowState, nil
}

func callRole(a *AssertRoot, method string) bool {
	return a.CallMethod(method, []any{"Роль", "документ", "Объект", "read", "описание"}).(bool)
}

func callField(a *AssertRoot, method string) bool {
	return a.CallMethod(method, []any{"Роль", "справочник", "Объект", "Поле", "описание"}).(bool)
}

// ── Матрица операций (план 112) ──────────────────────────────────────────────

func TestRoleAssert_NoChecker(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec) // checker не установлен — вне onebase test
	if callRole(a, "РольМожет") {
		t.Fatal("без checker ассерт должен провалиться")
	}
	if rec.last().Passed || rec.last().Detail == "" {
		t.Fatal("ожидался провал с пояснением")
	}
}

func TestRoleAssert_AllowedMatches(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	a.SetAccessChecker(fakeAccessChecker{allow: true})

	if !callRole(a, "РольМожет") || !rec.last().Passed {
		t.Fatal("РольМожет при allow=true должен пройти")
	}
	if callRole(a, "РольНеМожет") || rec.last().Passed {
		t.Fatal("РольНеМожет при allow=true должен провалиться")
	}
}

func TestRoleAssert_ResolveErrorFails(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	a.SetAccessChecker(fakeAccessChecker{err: errors.New("роль не найдена")})
	if callRole(a, "РольМожет") {
		t.Fatal("ошибка резолва должна проваливать ассерт")
	}
	if rec.last().Detail != "роль не найдена" {
		t.Fatalf("ожидался detail ошибки, получено %q", rec.last().Detail)
	}
}

// ── Полевой доступ (план 88) ─────────────────────────────────────────────────

func TestFieldMaskedAssert(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	a.SetAccessChecker(fakeAccessChecker{hasPolicy: true})

	if !callField(a, "ПолеМаскируется") || !rec.last().Passed {
		t.Fatal("ПолеМаскируется при hasPolicy=true должен пройти")
	}
	if callField(a, "ПолеВидно") || rec.last().Passed {
		t.Fatal("ПолеВидно при hasPolicy=true должен провалиться")
	}

	a.SetAccessChecker(fakeAccessChecker{hasPolicy: false})
	if !callField(a, "ПолеВидно") || !rec.last().Passed {
		t.Fatal("ПолеВидно при hasPolicy=false должен пройти")
	}
	if callField(a, "ПолеМаскируется") || rec.last().Passed {
		t.Fatal("ПолеМаскируется при hasPolicy=false должен провалиться")
	}
}

func TestMaskValueAssert(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	a.SetAccessChecker(fakeAccessChecker{masked: "••••••7890", hasPolicy: true})

	ok := a.CallMethod("МаскаПоля", []any{"Роль", "справочник", "Клиент", "Телефон", "1234567890", "••••••7890", "keep=4"}).(bool)
	if !ok || !rec.last().Passed {
		t.Fatal("МаскаПоля при совпадении должна пройти")
	}
	bad := a.CallMethod("МаскаПоля", []any{"Роль", "справочник", "Клиент", "Телефон", "1234567890", "9999", "неверно"}).(bool)
	if bad || rec.last().Passed {
		t.Fatal("МаскаПоля при расхождении должна провалиться")
	}
}

func TestFieldAssert_NoChecker(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	if callField(a, "ПолеМаскируется") {
		t.Fatal("без checker field-ассерт должен провалиться")
	}
	if rec.last().Detail == "" {
		t.Fatal("ожидалось пояснение")
	}
}

// ── Строковый доступ (план 79) ───────────────────────────────────────────────

func callRows(a *AssertRoot, method string) bool {
	return a.CallMethod(method, []any{"Роль", "документ", "Задача", "read", "описание"}).(bool)
}

func TestRowsAssert(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)

	a.SetAccessChecker(fakeAccessChecker{rowState: "restricted"})
	if !callRows(a, "СтрокиОграничены") || !rec.last().Passed {
		t.Fatal("СтрокиОграничены при restricted должен пройти")
	}
	if callRows(a, "СтрокиНеОграничены") || rec.last().Passed {
		t.Fatal("СтрокиНеОграничены при restricted должен провалиться")
	}

	a.SetAccessChecker(fakeAccessChecker{rowState: "unrestricted"})
	if !callRows(a, "СтрокиНеОграничены") || !rec.last().Passed {
		t.Fatal("СтрокиНеОграничены при unrestricted должен пройти")
	}

	// denied — обе проверки провал (сначала нужен РольМожет).
	a.SetAccessChecker(fakeAccessChecker{rowState: "denied"})
	if callRows(a, "СтрокиОграничены") || callRows(a, "СтрокиНеОграничены") {
		t.Fatal("при denied обе проверки строк должны провалиться")
	}
}

func TestRowsAssert_ResolveErrorFails(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	a.SetAccessChecker(fakeAccessChecker{err: errors.New("объект не найден")})
	if callRows(a, "СтрокиОграничены") {
		t.Fatal("ошибка резолва должна проваливать ассерт")
	}
	if rec.last().Detail != "объект не найден" {
		t.Fatalf("ожидался detail ошибки, получено %q", rec.last().Detail)
	}
}
