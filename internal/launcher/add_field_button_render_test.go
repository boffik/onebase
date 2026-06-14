package launcher

import (
	"bytes"
	"strings"
	"testing"
)

// Issue #65: кнопка «+ Добавить поле» в конфигураторе не реагировала на клик.
// Причина — в onclick имя сущности подставлялось как 3-й аргумент cfgAddField
// БЕЗ кавычек (text/template выводит {{$e.Name}} сырым), из-за чего браузер
// трактовал «Номенклатура» как необъявленную JS-переменную и падал с
// ReferenceError ещё до вызова функции. Фиксируем, что имя обёрнуто в кавычки —
// и для реквизитов шапки, и для полей табличной части.
func TestConfigurator_AddFieldButtonQuotesEntity(t *testing.T) {
	data := &configuratorData{
		Base: &Base{ID: "test-base", Name: "Тест", ConfigSource: "file"},
		Lang: "ru",
		Tab:  "tree",
		Catalogs: []cfgEntity{{
			Name: "Номенклатура", Kind: "Справочник",
			Fields: []cfgField{{Name: "Цена", Type: "number"}},
			TableParts: []cfgTablePart{{
				Name:   "Состав",
				Fields: []cfgField{{Name: "Количество", Type: "number"}},
			}},
		}},
	}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "tab-tree", data); err != nil {
		t.Fatalf("ExecuteTemplate tab-tree: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		// реквизиты шапки (строка-кнопка после таблицы полей)
		`cfgAddField('ft-Номенклатура','new_field','Номенклатура')`,
		// поля табличной части
		`cfgAddField('ft-Номенклатура-tp0','new_tp.Состав.field','Номенклатура')`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("в onclick кнопки «Добавить поле» нет ожидаемого фрагмента: %q", want)
		}
	}

	// И явно убеждаемся, что неэкранированный вариант (имя без кавычек,
	// вызывающий ReferenceError) больше не встречается.
	for _, bad := range []string{
		`'new_field',Номенклатура)`,
		`.field',Номенклатура)`,
	} {
		if strings.Contains(html, bad) {
			t.Errorf("в onclick осталось неэкранированное имя сущности: %q", bad)
		}
	}
}
