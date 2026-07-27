package ui

import (
	"strings"
	"testing"
)

func TestRefPickerDoesNotInjectOptionLabelAsHTML(t *testing.T) {
	src := string(uiJS)

	for _, bad := range []string{
		`+ opts[i].label +`,
		`+opts[i].label+`,
		`+ opts[j].label +`,
		`+opts[j].label+`,
	} {
		if strings.Contains(src, bad) {
			t.Fatalf("ref picker must not concatenate option label into innerHTML: found %q", bad)
		}
	}
	for _, want := range []string{
		`item.textContent = opts[i].label`,
		`opt.textContent = label`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ref picker should write option label through textContent: missing %q", want)
		}
	}
}

func TestRefPickerRejectsReadOnlyTargetBeforeOpening(t *testing.T) {
	src := string(uiJS)
	start := strings.Index(src, "function openRefPicker(selOrId)")
	end := strings.Index(src, "function openRefCurrent(selOrId)")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("openRefPicker function not found in ui.js")
	}
	picker := src[start:end]

	guard := strings.Index(picker, "sel.disabled || sel.readOnly || sel.hasAttribute('readonly')")
	firstRead := strings.Index(picker, "sel.getAttribute(")
	if guard < 0 {
		t.Fatal("openRefPicker must reject disabled and readonly targets")
	}
	if firstRead < 0 || guard > firstRead {
		t.Fatal("openRefPicker must check target editability before reading picker attributes")
	}
}
