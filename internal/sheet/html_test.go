package sheet

import (
	"strings"
	"testing"
)

// TestSafeColor — валидные цвета проходят, невалидные (с кавычками / тегами) — отбрасываются.
func TestSafeColor(t *testing.T) {
	cases := []struct {
		in   string
		want string // "" = должен быть отброшен
	}{
		{"#ff0000", "#ff0000"},
		{"#f00", "#f00"},
		{"#aabbccdd", "#aabbccdd"},
		{"red", "red"},
		{"dark-blue", "dark-blue"},
		{"AliceBlue", "AliceBlue"},
		// инъекции — должны быть отброшены
		{`red"><script>`, ""},
		{`#fff" style="x:y`, ""},
		{"red; color:red", ""},
		{"url(evil)", ""},
		{"rgba(0,0,0,0)", ""},
	}
	for _, c := range cases {
		got := safeColor(c.in)
		if got != c.want {
			t.Errorf("safeColor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSafeFontFamily — вырезаются " ' < > ;
func TestSafeFontFamily(t *testing.T) {
	if got := safeFontFamily(`Arial`); got != "Arial" {
		t.Errorf("Arial → %q", got)
	}
	if got := safeFontFamily(`Arial"><script>`); strings.ContainsAny(got, `"'<>;`) {
		t.Errorf("safeFontFamily не вырезал спецсимволы: %q", got)
	}
	if got := safeFontFamily(`Times New Roman`); got != "Times New Roman" {
		t.Errorf("Times New Roman → %q", got)
	}
}

// TestHTMLStyleNoAttributeBreak — BackColor/TextColor с инъекцией не ломают style-атрибут.
func TestHTMLStyleNoAttributeBreak(t *testing.T) {
	d := NewDocument()
	c := d.GetOrCreateCell(0, 0)
	c.Text = "Привет"
	c.BackColor = `red"><script>alert(1)</script>`
	c.TextColor = `blue' onload='evil()`
	c.FontFamily = `Arial"; x-inject: bad`

	html := d.HTML(HTMLOptions{})

	// Не должно быть разрыва атрибута через "> или последовательности, закрывающей style.
	for _, bad := range []string{`"><`, `'<`, `</script>`} {
		if strings.Contains(html, bad) {
			t.Errorf("HTML содержит небезопасную последовательность %q:\n%s", bad, html)
		}
	}
	// Тег script не должен появиться в выводе.
	if strings.Contains(strings.ToLower(html), "<script") {
		t.Errorf("HTML содержит <script> тег из инъекции")
	}
}

// TestHTMLStyleValidColorPassthrough — валидные цвета не теряются.
func TestHTMLStyleValidColorPassthrough(t *testing.T) {
	d := NewDocument()
	c := d.GetOrCreateCell(0, 0)
	c.Text = "Ячейка"
	c.BackColor = "#eef"
	c.TextColor = "darkred"
	c.FontFamily = "Arial"

	html := d.HTML(HTMLOptions{})

	for _, want := range []string{"background-color:#eef", "color:darkred", "font-family:Arial"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML не содержит ожидаемый стиль %q", want)
		}
	}
}
