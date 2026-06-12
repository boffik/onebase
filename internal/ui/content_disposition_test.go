package ui

import (
	"strings"
	"testing"
)

// Issue #46: сырой UTF-8 в filename="..." декодируется браузером как latin-1 —
// имя файла превращается в кракозябры. RFC 6266: ASCII-фолбэк в filename= и
// полное имя в filename*=UTF-8''.
func TestContentDisposition(t *testing.T) {
	got := contentDisposition("Контрагенты.xlsx")
	if !strings.Contains(got, "filename*=UTF-8''") {
		t.Fatalf("нет RFC 5987 параметра: %q", got)
	}
	if strings.Contains(got, `filename="Контрагенты`) {
		t.Fatalf("сырой UTF-8 в quoted-string остался: %q", got)
	}
	if !strings.HasPrefix(got, "attachment; ") {
		t.Fatalf("ожидался attachment: %q", got)
	}
	// ASCII-имя остаётся читаемым в обоих параметрах.
	got = contentDisposition("report.pdf")
	if !strings.Contains(got, `filename="report.pdf"`) {
		t.Fatalf("ASCII-фолбэк потерян: %q", got)
	}
}
