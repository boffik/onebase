package launcher

import (
	"strings"
	"testing"
)

// Регрессия для бага «кнопки админки не нажимаются»: узбекское «O'zbekcha»
// попадало в JS-литерал {l:'O'zbekcha'} без экранирования, апостроф
// закрывал строку, и весь скрипт админ-панели не парсился. Тест дублирует
// логику экранирования из cfgAdminUsers и фиксирует её поведение.
func TestJSEscape_Apostrophe(t *testing.T) {
	jsEscape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		return strings.ReplaceAll(s, `'`, `\'`)
	}
	cases := []struct {
		in, want string
	}{
		{"O'zbekcha", `O\'zbekcha`},
		{"Plain", "Plain"},
		{"back\\slash", `back\\slash`},
		{"both ' and \\", `both \' and \\`},
	}
	for _, c := range cases {
		if got := jsEscape(c.in); got != c.want {
			t.Errorf("jsEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
