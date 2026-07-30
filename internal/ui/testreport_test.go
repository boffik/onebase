package ui

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
)

// sampleResult — прогон со всеми исходами: pass, fail, error, empty.
func sampleResult() TestRunResult {
	return TestRunResult{Cases: []TestCaseResult{
		{
			Name: "PassT", Passed: 2, Duration: 2 * time.Millisecond,
			Asserts: []interpreter.AssertOutcome{{Passed: true, Desc: "a"}, {Passed: true, Desc: "b"}},
		},
		{
			Name: "FailT", Passed: 1, Failed: 1, Duration: time.Millisecond,
			Asserts: []interpreter.AssertOutcome{
				{Passed: true, Desc: "ок"},
				{Passed: false, Desc: "провал", Detail: "ожидалось «2», получено «1»"},
			},
		},
		{
			Name: "ErrT", Passed: 0, Err: errors.New("бум"),
			Asserts: []interpreter.AssertOutcome{},
		},
		{
			Name: "EmptyT", Asserts: nil,
		},
	}}
}

func TestWriteReport_TAP(t *testing.T) {
	var b strings.Builder
	if err := WriteReport(&b, sampleResult(), FormatTAP); err != nil {
		t.Fatalf("WriteReport tap: %v", err)
	}
	out := b.String()
	for _, want := range []string{
		"TAP version 13",
		"1..4",
		"ok 1 - PassT",
		"not ok 2 - FailT",
		"not ok 3 - ErrT",
		"not ok 4 - EmptyT",
		"error:", // диагностика ошибки ErrT
		"провал", // диагностика провала FailT
	} {
		if !strings.Contains(out, want) {
			t.Errorf("TAP не содержит %q\n---\n%s", want, out)
		}
	}
}

func TestWriteReport_JUnitValidAndCounts(t *testing.T) {
	var b strings.Builder
	if err := WriteReport(&b, sampleResult(), FormatJUnit); err != nil {
		t.Fatalf("WriteReport junit: %v", err)
	}
	var doc junitTestsuites
	if err := xml.Unmarshal([]byte(b.String()), &doc); err != nil {
		t.Fatalf("JUnit — невалидный XML: %v\n%s", err, b.String())
	}
	if doc.Tests != 4 {
		t.Errorf("tests=%d, ожидалось 4", doc.Tests)
	}
	// FailT и EmptyT → failures; ErrT → error.
	if doc.Failures != 2 {
		t.Errorf("failures=%d, ожидалось 2", doc.Failures)
	}
	if doc.Errors != 1 {
		t.Errorf("errors=%d, ожидалось 1", doc.Errors)
	}
	if len(doc.Suites) != 1 || len(doc.Suites[0].Cases) != 4 {
		t.Fatalf("ожидался 1 suite с 4 testcase, получено %+v", doc.Suites)
	}
	// У проваленного теста есть <failure>, у ошибочного — <error>.
	byName := map[string]junitCase{}
	for _, c := range doc.Suites[0].Cases {
		byName[c.Name] = c
	}
	if byName["FailT"].Failure == nil {
		t.Error("FailT должен нести <failure>")
	}
	if byName["ErrT"].Error == nil {
		t.Error("ErrT должен нести <error>")
	}
	if byName["PassT"].Failure != nil || byName["PassT"].Error != nil {
		t.Error("PassT не должен нести failure/error")
	}
}

func TestWriteReport_Pretty(t *testing.T) {
	var b strings.Builder
	if err := WriteReport(&b, sampleResult(), FormatPretty); err != nil {
		t.Fatalf("WriteReport pretty: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, "── Итог ──") || !strings.Contains(out, "Тестов: 4") {
		t.Errorf("pretty без итога:\n%s", out)
	}
}

func TestWriteReport_UnknownFormat(t *testing.T) {
	var b strings.Builder
	if err := WriteReport(&b, sampleResult(), "xml"); err == nil {
		t.Fatal("неизвестный формат должен вернуть ошибку")
	}
}
