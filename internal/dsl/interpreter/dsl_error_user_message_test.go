package interpreter_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
)

func TestDSLError_UserMessage_omitsFileLine(t *testing.T) {
	err := &interpreter.DSLError{
		File: "/root/controlling/demo-app/src/показаниясчётчика.posting.os",
		Line: 60,
		Msg:  "Показание уже зафиксировано, сначала отмените проведение",
	}
	got := err.UserMessage()
	if got != err.Msg {
		t.Fatalf("UserMessage = %q, want %q", got, err.Msg)
	}
	if strings.Contains(got, ".os:") || strings.Contains(got, "/root/") {
		t.Fatalf("UserMessage must not expose source path: %q", got)
	}
	debug := err.Error()
	if !strings.Contains(debug, ".posting.os:60:") {
		t.Fatalf("Error() must keep file:line for tools, got %q", debug)
	}
}

func TestFormatUserError_DSLErrorAndPlain(t *testing.T) {
	dsl := &interpreter.DSLError{File: "x.os", Line: 1, Msg: "только текст"}
	if got := interpreter.FormatUserError(dsl); got != "только текст" {
		t.Fatalf("FormatUserError(DSLError) = %q", got)
	}
	wrapped := fmt.Errorf("wrap: %w", dsl)
	if got := interpreter.FormatUserError(wrapped); got != "только текст" {
		t.Fatalf("FormatUserError(wrapped) = %q", got)
	}
	plain := errors.New("техническая ошибка")
	if got := interpreter.FormatUserError(plain); got != "техническая ошибка" {
		t.Fatalf("FormatUserError(plain) = %q", got)
	}
	if got := interpreter.FormatUserError(nil); got != "" {
		t.Fatalf("FormatUserError(nil) = %q", got)
	}
}
