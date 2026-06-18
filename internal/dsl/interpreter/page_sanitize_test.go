package interpreter

import (
	"strings"
	"testing"
)

// sanitizePageHTML должен убирать активный контент из произвольного HTML блока
// ДобавитьСыройHTML (план 66). Прежний блоклист на регулярках обходился через
// разделитель "/" перед обработчиком, data:-URI и xlink:href — проверяем именно
// эти векторы плюс базовые.
func TestSanitizePageHTML_StripsActiveContent(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		mustNotHave []string
	}{
		{"script", `<p>ок</p><script>alert(1)</script>`, []string{"<script", "alert(1)"}},
		{"onerror-space", `<img src=x onerror=alert(1)>`, []string{"onerror"}},
		{"onerror-slash", `<img/onerror=alert(1)>`, []string{"onerror"}},
		{"svg-onload", `<svg/onload=alert(1)></svg>`, []string{"onload"}},
		{"js-uri", `<a href="javascript:alert(1)">x</a>`, []string{"javascript:"}},
		{"js-uri-tab", "<a href=\"java\tscript:alert(1)\">x</a>", []string{"script:alert"}},
		{"data-iframe", `<iframe src="data:text/html,<script>alert(1)</script>"></iframe>`, []string{"<iframe", "data:text/html"}},
		{"xlink", `<svg><a xlink:href="javascript:alert(1)"><text>x</text></a></svg>`, []string{"javascript:"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := sanitizePageHTML(c.in)
			low := strings.ToLower(out)
			for _, bad := range c.mustNotHave {
				if strings.Contains(low, strings.ToLower(bad)) {
					t.Errorf("санитайзер пропустил %q\n  вход:  %s\n  выход: %s", bad, c.in, out)
				}
			}
		})
	}
}

// Безопасное форматирование должно сохраняться — иначе ДобавитьСыройHTML теряет смысл.
func TestSanitizePageHTML_KeepsSafeMarkup(t *testing.T) {
	out := sanitizePageHTML(`<h3>Итоги</h3><p>Выручка <b>100</b></p><table><tr><td>A</td></tr></table>`)
	for _, want := range []string{"<h3", "Итоги", "<b>", "<table", "<td"} {
		if !strings.Contains(out, want) {
			t.Errorf("санитайзер вырезал безопасное %q: %s", want, out)
		}
	}
}
