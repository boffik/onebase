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
