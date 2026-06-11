package metadata

import "testing"

func TestValidIdent(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Номенклатура", true},
		{"Counterparty", true},
		{"СтавкаНДС", true},
		{"_служебное", true},
		{"Поле1", true},
		{"a", true},

		{"", false},
		{"Дата платежа", false}, // пробел
		{"Поле-2", false},       // дефис
		{"1Поле", false},        // начинается с цифры
		{"Контрагент;DROP", false},
		{`Имя"`, false},
		{"Контр.агент", false}, // точка
		{"Сумма(руб)", false},  // скобки
	}
	for _, c := range cases {
		if got := ValidIdent(c.name); got != c.want {
			t.Errorf("ValidIdent(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestValidateIdentifiers_OK(t *testing.T) {
	entities := []*Entity{
		{Name: "Контрагент", Kind: KindCatalog, Fields: []Field{{Name: "Наименование", Type: FieldTypeString}}},
		{
			Name: "Поступление", Kind: KindDocument,
			Fields:     []Field{{Name: "Номер", Type: FieldTypeString}},
			TableParts: []TablePart{{Name: "Товары", Fields: []Field{{Name: "Количество", Type: FieldTypeNumber}}}},
		},
	}
	registers := []*Register{{
		Name:       "ОстаткиТоваров",
		Dimensions: []Field{{Name: "Номенклатура", Type: FieldTypeString}},
		Resources:  []Field{{Name: "Количество", Type: FieldTypeNumber}},
	}}
	inforegs := []*InfoRegister{{
		Name:       "ЦеныНоменклатуры",
		Dimensions: []Field{{Name: "Номенклатура", Type: FieldTypeString}},
		Resources:  []Field{{Name: "Цена", Type: FieldTypeNumber}},
	}}
	enums := []*Enum{{Name: "ВидКонтрагента", Values: []string{"Поставщик"}}}
	constants := []*Constant{{Name: "ОсновнаяВалюта", Type: FieldTypeString}}

	if err := ValidateIdentifiers(entities, registers, inforegs, nil, enums, constants); err != nil {
		t.Fatalf("корректная конфигурация не должна давать ошибку: %v", err)
	}
}

func TestValidateIdentifiers_Rejects(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{"имя справочника с пробелом", func() error {
			return ValidateIdentifiers([]*Entity{{Name: "Мой Справочник", Kind: KindCatalog}}, nil, nil, nil, nil, nil)
		}},
		{"реквизит с дефисом", func() error {
			e := &Entity{Name: "Контрагент", Kind: KindCatalog, Fields: []Field{{Name: "ИНН-КПП", Type: FieldTypeString}}}
			return ValidateIdentifiers([]*Entity{e}, nil, nil, nil, nil, nil)
		}},
		{"реквизит совпадает со служебной колонкой", func() error {
			e := &Entity{Name: "Контрагент", Kind: KindCatalog, Fields: []Field{{Name: "id", Type: FieldTypeString}}}
			return ValidateIdentifiers([]*Entity{e}, nil, nil, nil, nil, nil)
		}},
		{"табличная часть с инъекцией", func() error {
			e := &Entity{Name: "Поступление", Kind: KindDocument, TableParts: []TablePart{{Name: "Товары; DROP TABLE"}}}
			return ValidateIdentifiers([]*Entity{e}, nil, nil, nil, nil, nil)
		}},
		{"измерение регистра с кавычкой", func() error {
			r := &Register{Name: "Остатки", Dimensions: []Field{{Name: `Ном"`, Type: FieldTypeString}}}
			return ValidateIdentifiers(nil, []*Register{r}, nil, nil, nil, nil)
		}},
		{"перечисление с точкой", func() error {
			return ValidateIdentifiers(nil, nil, nil, nil, []*Enum{{Name: "Вид.Контрагента"}}, nil)
		}},
		{"константа начинается с цифры", func() error {
			return ValidateIdentifiers(nil, nil, nil, nil, nil, []*Constant{{Name: "1Валюта"}})
		}},
	}
	for _, tt := range tests {
		if err := tt.run(); err == nil {
			t.Errorf("%s: ожидалась ошибка валидации, получено nil", tt.name)
		}
	}
}
