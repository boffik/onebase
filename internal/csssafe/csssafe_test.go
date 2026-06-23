package csssafe

import "testing"

func TestColor(t *testing.T) {
	for _, c := range []string{"#c00", "#cc0000", "#cc0000ff", "rgb(255,0,0)", "rgba(255, 0, 0, .5)", "red", "transparent"} {
		if got := Color(c); got != c {
			t.Fatalf("Color(%q) = %q", c, got)
		}
	}
	for _, c := range []string{"red;background:url(javascript:1)", "#c00;body{}", "url(x)", "expression(x)", "нечто"} {
		if got := Color(c); got != "" {
			t.Fatalf("Color(%q) = %q, want empty", c, got)
		}
	}
}

func TestLength(t *testing.T) {
	for _, v := range []string{"0", "10px", "12.5pt", "100%", "auto"} {
		if got := Length(v); got != v {
			t.Fatalf("Length(%q) = %q", v, got)
		}
	}
	for _, v := range []string{`10px;color:red`, `url(x)`, `calc(100%)`, `auto;background:red`} {
		if got := Length(v); got != "" {
			t.Fatalf("Length(%q) = %q, want empty", v, got)
		}
	}
}

func TestFontFamily(t *testing.T) {
	if got := FontFamily(`Arial";color:red`); got != "Arialcolor:red" {
		t.Fatalf("FontFamily stripped to %q", got)
	}
}

func TestTextAlign(t *testing.T) {
	if got := TextAlign(" Center "); got != "center" {
		t.Fatalf("TextAlign = %q", got)
	}
	if got := TextAlign("left;position:absolute"); got != "" {
		t.Fatalf("TextAlign injected = %q", got)
	}
}
