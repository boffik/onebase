package ui

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

// Сигнатуры реальной разметки Lucide — чтобы проверить, что рендерится именно
// нужная иконка, а не просто «какой-то svg».
const (
	sigShoppingCart = "M2.05 2.05h2l2.66" // фрагмент пути shopping-cart
	sigSquare       = `<rect width="18" height="18" x="3" y="3" rx="2"`
)

func TestLucideIcon_Known(t *testing.T) {
	got := string(LucideIcon("shopping-cart"))
	if got == "" {
		t.Fatal("LucideIcon(shopping-cart) пустой")
	}
	if !strings.Contains(got, "<svg") || !strings.Contains(got, "</svg>") {
		t.Errorf("нет обёртки <svg>: %q", got)
	}
	if !strings.Contains(got, `class="lucide ob-icon"`) {
		t.Errorf("нет класса lucide ob-icon: %q", got)
	}
	if !strings.Contains(got, sigShoppingCart) {
		t.Errorf("нет пути shopping-cart: %q", got)
	}
}

func TestLucideIcon_EmptyIsEmpty(t *testing.T) {
	if got := LucideIcon(""); got != "" {
		t.Errorf("LucideIcon(\"\") = %q, ожидалась пустая строка", got)
	}
	if got := LucideIcon("   "); got != "" {
		t.Errorf("LucideIcon(пробелы) = %q, ожидалась пустая строка", got)
	}
}

func TestLucideIcon_UnknownFallsBackToSquare(t *testing.T) {
	got := string(LucideIcon("definitely-not-an-icon-xyz"))
	if got == "" {
		t.Fatal("неизвестная иконка дала пустую строку (нет фолбэка)")
	}
	if !strings.Contains(got, sigSquare) {
		t.Errorf("фолбэк не похож на square: %q", got)
	}
	// Битой разметки быть не должно: ни пустого href, ни оборванного svg.
	if strings.Contains(got, `href=""`) {
		t.Errorf("в фолбэке пустой href: %q", got)
	}
}

func TestLucideIcon_CaseAndTrim(t *testing.T) {
	want := LucideIcon("shopping-cart")
	for _, in := range []string{"Shopping-Cart", "  shopping-cart  ", "SHOPPING-CART"} {
		if got := LucideIcon(in); got != want {
			t.Errorf("LucideIcon(%q) не совпал с каноничным", in)
		}
	}
}

func TestLucideIcon_Aliases(t *testing.T) {
	cases := [][2]string{
		{"home", "house"},
		{"cart", "shopping-cart"},
		{"ruble", "russian-ruble"},
		{"bar-chart-3", "chart-column"},
		{"pie-chart", "chart-pie"},
	}
	for _, c := range cases {
		if LucideIcon(c[0]) != LucideIcon(c[1]) {
			t.Errorf("синоним %q не разрешился в %q", c[0], c[1])
		}
		if LucideIcon(c[0]) == "" {
			t.Errorf("синоним %q дал пустую иконку", c[0])
		}
	}
}

func TestNormalizeIconName(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"   ":                   "",
		"shopping-cart":         "shopping-cart",
		"Shopping Cart":         "shopping-cart",
		"shopping_cart":         "shopping-cart",
		"  Layout--Dashboard  ": "layout-dashboard",
		"LAYOUT DASHBOARD":      "layout-dashboard",
		"-leading-dash":         "leading-dash",
		"trailing-dash-":        "trailing-dash",
	}
	for in, want := range cases {
		if got := NormalizeIconName(in); got != want {
			t.Errorf("NormalizeIconName(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestLucideNames_SortedAndCanonical(t *testing.T) {
	names := LucideNames()
	if len(names) == 0 {
		t.Fatal("список имён пуст")
	}
	if !sort.StringsAreSorted(names) {
		t.Error("список имён не отсортирован")
	}
	idx := make(map[string]bool, len(names))
	for _, n := range names {
		idx[n] = true
	}
	for _, must := range []string{"square", "shopping-cart", "layout-dashboard"} {
		if !idx[must] {
			t.Errorf("в списке нет каноничного имени %q", must)
		}
	}
	// Синонимы в подсказку не попадают — только каноничные имена.
	if idx["home"] {
		t.Error("в списке имён не должно быть синонима home")
	}
}

func TestLucideIconsJSON_ParsesAndAliasesMatch(t *testing.T) {
	raw := string(LucideIconsJSON())
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("LucideIconsJSON не парсится как JSON: %v", err)
	}
	if m["shopping-cart"] == "" || m["square"] == "" {
		t.Error("в JSON нет каноничных иконок")
	}
	// Синоним должен присутствовать и указывать на ту же разметку, что канон.
	if m["home"] == "" || m["home"] != m["house"] {
		t.Errorf("синоним home в JSON не совпадает с house")
	}
	// Безопасность встраивания в <script>: < экранирован как <, сырых < нет.
	if strings.Contains(raw, "<") {
		t.Errorf("в JSON есть сырой символ < (небезопасно для <script>)")
	}
}

// TestSubsysBar_RendersIcons — основной тест плана 72: панель подсистем выводит
// иконку перед заголовком; пустое имя иконки не даёт лишней разметки; неизвестное
// имя сворачивается в фолбэк (square).
func TestSubsysBar_RendersIcons(t *testing.T) {
	data := map[string]any{
		"Cfg":              Config{},
		"Lang":             "ru",
		"IsAdmin":          false,
		"CurrentSubsystem": "",
		"Subsystems": []*metadata.Subsystem{
			{Name: "Продажи", Title: "Продажи", Icon: "shopping-cart"},
			{Name: "Склад", Title: "Склад", Icon: ""},              // пусто → без иконки
			{Name: "Прочее", Title: "Прочее", Icon: "no-such-xyz"}, // неизвестно → square
		},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "nav", data); err != nil {
		t.Fatalf("execute nav: %v", err)
	}
	html := buf.String()

	if !strings.Contains(html, `<nav class="subsys-bar">`) {
		t.Fatal("нет панели подсистем")
	}
	// Заголовки всех подсистем на месте.
	for _, name := range []string{"Продажи", "Склад", "Прочее"} {
		if !strings.Contains(html, name) {
			t.Errorf("в панели нет подсистемы %q", name)
		}
	}
	// Реальная иконка корзины отрисована.
	if !strings.Contains(html, sigShoppingCart) {
		t.Error("иконка shopping-cart не отрисована в панели")
	}
	// Неизвестное имя свернулось в square (фолбэк).
	if !strings.Contains(html, sigSquare) {
		t.Error("неизвестная иконка не дала фолбэк square")
	}
	// Ровно две иконки: shopping-cart и фолбэк. Подсистема с пустым icon иконку
	// не рисует (иначе была бы битая/лишняя разметка).
	if n := strings.Count(html, "lucide ob-icon"); n != 2 {
		t.Errorf("ожидалось 2 иконки в панели, получено %d", n)
	}
}
