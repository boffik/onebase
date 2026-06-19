package query

import "testing"

// TestSourcePermKind: тип источника → секция прав User.Has. Регистр бухгалтерии
// маппится на "register" (фикс зазора плана 54), а не на "" (иначе rbac запрещает
// его не-админам всегда).
func TestSourcePermKind(t *testing.T) {
	cases := map[string]string{
		"СПРАВОЧНИК":         "catalog",
		"CATALOG":            "catalog",
		"ДОКУМЕНТ":           "document",
		"DOCUMENT":           "document",
		"РЕГИСТРБУХГАЛТЕРИИ": "register",
		"ACCOUNTINGREGISTER": "register",
		"НЕЧТО":              "",
	}
	for in, want := range cases {
		if got := sourcePermKind(in); got != want {
			t.Errorf("sourcePermKind(%q)=%q, хотим %q", in, got, want)
		}
	}
}
