package version

import (
	"runtime/debug"
	"testing"
	"time"
)

func TestParseVCS(t *testing.T) {
	settings := []debug.BuildSetting{
		{Key: "vcs.revision", Value: "cb5276e9c523366f3e91fbf2457aa1a9941e0f3b"},
		{Key: "vcs.time", Value: "2026-06-25T11:01:36Z"},
		{Key: "vcs.modified", Value: "true"},
		{Key: "GOARCH", Value: "amd64"}, // посторонняя настройка игнорируется
	}
	d := parseVCS(settings)
	if d.revision != "cb5276e9c523366f3e91fbf2457aa1a9941e0f3b" {
		t.Errorf("revision = %q", d.revision)
	}
	if !d.modified {
		t.Error("modified = false, want true")
	}
	want := time.Date(2026, 6, 25, 11, 1, 36, 0, time.UTC)
	if !d.time.Equal(want) {
		t.Errorf("time = %v, want %v", d.time, want)
	}
}

func TestParseVCSEmptyAndBad(t *testing.T) {
	d := parseVCS(nil)
	if d.revision != "" || d.modified || !d.time.IsZero() {
		t.Errorf("parseVCS(nil) = %+v, want zero", d)
	}
	// Некорректное время не должно паниковать и оставляет нулевой time.
	d = parseVCS([]debug.BuildSetting{{Key: "vcs.time", Value: "not-a-date"}})
	if !d.time.IsZero() {
		t.Errorf("time = %v, want zero on bad input", d.time)
	}
}

func TestShortRev(t *testing.T) {
	cases := map[string]string{
		"cb5276e9c523366f3e91fbf2457aa1a9941e0f3b": "cb5276e",
		"abc": "",
		"":    "",
	}
	for in, want := range cases {
		if got := shortRev(in); got != want {
			t.Errorf("shortRev(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFmtDate(t *testing.T) {
	d := time.Date(2026, 6, 25, 11, 1, 36, 0, time.UTC)
	if got := fmtDate(d); got != "25.06.26" {
		t.Errorf("fmtDate = %q, want 25.06.26", got)
	}
	if got := fmtDate(time.Time{}); got != "" {
		t.Errorf("fmtDate(zero) = %q, want empty", got)
	}
}
