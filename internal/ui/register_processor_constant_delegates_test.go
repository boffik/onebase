package ui

import (
	"strings"
	"testing"
)

func TestRegisterProcessorConstantDelegatedHandlers(t *testing.T) {
	html := tplRegister + tplDeleteMarked + tplProcessor + tplInfoReg + tplConstants
	for _, want := range []string{
		`data-ob-ref-picker="regflt-`,
		`data-ob-confirm="{{t $.Lang "Удалить все помеченные записи без ссылок?"}}"`,
		`data-ob-ref-picker="pp-`,
		`data-ob-ref-current="pp-`,
		`data-ob-confirm="{{t $.Lang "Удалить запись?"}}"`,
		`data-ob-ref-picker="ird-`,
		`data-ob-ref-current="ird-`,
		`data-ob-ref-picker="const-`,
		`data-ob-ref-current="const-`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("templates do not contain delegated marker %q", want)
		}
	}
	for _, old := range []string{
		`onclick="openRefPicker('regflt-`,
		`onclick="openRefPicker('pp-`,
		`onclick="openRefCurrent('pp-`,
		`onclick="openRefPicker('ird-`,
		`onclick="openRefCurrent('ird-`,
		`onclick="openRefPicker('const-`,
		`onclick="openRefCurrent('const-`,
		`onsubmit="return confirm('{{t $.Lang "Удалить все помеченные записи без ссылок?"}}')"`,
		`onsubmit="return confirm('{{t $.Lang "Удалить запись?"}}')"`,
	} {
		if strings.Contains(html, old) {
			t.Fatalf("templates still contain inline handler %q", old)
		}
	}
}
