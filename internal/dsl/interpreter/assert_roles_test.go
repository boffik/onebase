package interpreter

import (
	"errors"
	"testing"
)

type capRecorder struct{ outcomes []AssertOutcome }

func (c *capRecorder) RecordAssert(o AssertOutcome) { c.outcomes = append(c.outcomes, o) }

func (c *capRecorder) last() AssertOutcome { return c.outcomes[len(c.outcomes)-1] }

type fakeRoleChecker struct {
	allow bool
	err   error
}

func (f fakeRoleChecker) RoleAllows(_, _, _, _ string) (bool, error) { return f.allow, f.err }

func callRole(a *AssertRoot, method string) bool {
	return a.CallMethod(method, []any{"Роль", "документ", "Объект", "read", "описание"}).(bool)
}

func TestRoleAssert_NoChecker(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec) // checker не установлен — вне onebase test
	if callRole(a, "РольМожет") {
		t.Fatal("без checker ассерт должен провалиться")
	}
	if rec.last().Passed {
		t.Fatal("ожидался провал")
	}
	if rec.last().Detail == "" {
		t.Fatal("ожидалось пояснение, почему проверка недоступна")
	}
}

func TestRoleAssert_AllowedMatches(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	a.SetRoleChecker(fakeRoleChecker{allow: true})

	if !callRole(a, "РольМожет") {
		t.Fatal("РольМожет при allow=true должен пройти")
	}
	if !rec.last().Passed {
		t.Fatal("ожидался успех РольМожет")
	}

	if callRole(a, "РольНеМожет") {
		t.Fatal("РольНеМожет при allow=true должен провалиться")
	}
	if rec.last().Passed {
		t.Fatal("ожидался провал РольНеМожет")
	}
}

func TestRoleAssert_DeniedMatches(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	a.SetRoleChecker(fakeRoleChecker{allow: false})

	if callRole(a, "РольНеМожет") == false {
		t.Fatal("РольНеМожет при allow=false должен пройти")
	}
	if !rec.last().Passed {
		t.Fatal("ожидался успех РольНеМожет")
	}
}

func TestRoleAssert_ResolveErrorFails(t *testing.T) {
	rec := &capRecorder{}
	a := NewAssertRoot(rec)
	a.SetRoleChecker(fakeRoleChecker{err: errors.New("роль не найдена")})

	if callRole(a, "РольМожет") {
		t.Fatal("ошибка резолва должна проваливать ассерт")
	}
	if got := rec.last().Detail; got != "роль не найдена" {
		t.Fatalf("ожидался detail с текстом ошибки, получено %q", got)
	}
}
