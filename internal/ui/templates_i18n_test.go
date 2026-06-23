package ui

import (
	"testing"
	"testing/fstest"

	"github.com/ivantit66/onebase/internal/i18n"
)

func mustBundle(t *testing.T, translation string) *i18n.Bundle {
	t.Helper()
	b, err := i18n.Load(fstest.MapFS{
		"en.json": {Data: []byte(`{"Сохранить":` + translation + `}`)},
	}, "")
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	return b
}

func TestTemplateFuncsUseCapturedBundle(t *testing.T) {
	first := templateFuncs(mustBundle(t, `"Save"`))["t"].(func(string, string) string)
	second := templateFuncs(mustBundle(t, `"Store"`))["t"].(func(string, string) string)

	if got := first("en", "Сохранить"); got != "Save" {
		t.Fatalf("first bundle translation = %q", got)
	}
	if got := second("en", "Сохранить"); got != "Store" {
		t.Fatalf("second bundle translation = %q", got)
	}
	if got := first("en", "Сохранить"); got != "Save" {
		t.Fatalf("first bundle was affected by second template: %q", got)
	}
}
