package aicontext

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
)

func TestSchemaText_Entities(t *testing.T) {
	doc := &metadata.Entity{
		Name: "Заказ", Kind: metadata.KindDocument, Posting: true,
		Fields: []metadata.Field{{Name: "Дата", Type: metadata.FieldTypeDate}},
		TableParts: []metadata.TablePart{
			{Name: "Товары", Fields: []metadata.Field{{Name: "Количество", Type: metadata.FieldTypeNumber}}},
		},
		Forms: []*metadata.FormModule{{Name: "ФормаЗаказа", Kind: "object"}},
	}
	cat := &metadata.Entity{
		Name: "Клиент", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	txt := SchemaText(Input{
		Entities: []*metadata.Entity{doc, cat},
		Enums:    []*metadata.Enum{{Name: "Статус", Values: []string{"Новый", "Закрыт"}}},
		Reports:  []NamedTitle{{Name: "Продажи", Title: "Отчёт продаж"}},
	})
	for _, sub := range []string{
		"Документы:", "Заказ", "(проводится)", "ТЧ Товары", "Количество",
		"формы: ФормаЗаказа (object)", "Справочники:", "Клиент", "Наименование",
		"Перечисления:", "Статус", "Закрыт", "Отчёты", "Продажи — Отчёт продаж",
	} {
		if !strings.Contains(txt, sub) {
			t.Errorf("в срезе нет %q:\n%s", sub, txt)
		}
	}
}

func TestSchemaText_Empty(t *testing.T) {
	if got := SchemaText(Input{}); !strings.Contains(got, "нет объектов") {
		t.Errorf("ожидалась заглушка для пустого Input, получено %q", got)
	}
}
