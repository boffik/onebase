package configcheck

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/project"
)

// projWithElement собирает минимальный проект с одной формой сущности
// «ВходящееПисьмо» и единственным элементом el.
func projWithElement(el *metadata.FormElement) *project.Project {
	return &project.Project{
		Entities: []*metadata.Entity{
			{
				Name: "ВходящееПисьмо",
				Forms: []*metadata.FormModule{
					{Name: "объекта", Elements: []*metadata.FormElement{el}},
				},
			},
		},
	}
}

func TestCheckFormFieldFormat_PoleDatyFormatWarns(t *testing.T) {
	warns := CheckFormFieldFormat(projWithElement(&metadata.FormElement{
		Kind:     metadata.FormElementDatePicker,
		Name:     "Дата",
		DataPath: "Объект.Дата",
		Format:   "dd.MM.yyyy",
	}))
	if len(warns) != 1 {
		t.Fatalf("ожидалось 1 предупреждение, получили %d: %+v", len(warns), warns)
	}
	w := warns[0]
	if w.Code != "form.format-ignored" {
		t.Errorf("Code = %q, ожидался form.format-ignored", w.Code)
	}
	if w.File != "forms/входящееписьмо/объекта.form.yaml" {
		t.Errorf("File = %q, ожидался путь формы в нижнем регистре", w.File)
	}
	if !strings.Contains(w.Message, "ПолеДаты") || !strings.Contains(w.Message, "ISO") {
		t.Errorf("сообщение должно объяснять суть (ПолеДаты/ISO): %q", w.Message)
	}
}

func TestCheckFormFieldFormat_DisplayFormatAlsoWarns(t *testing.T) {
	if warns := CheckFormFieldFormat(projWithElement(&metadata.FormElement{
		Kind:          metadata.FormElementDatePicker,
		Name:          "Дата",
		DisplayFormat: "dd.MM.yyyy",
	})); len(warns) != 1 {
		t.Fatalf("display_format тоже должен предупреждать, получили %d", len(warns))
	}
}

func TestCheckFormFieldFormat_NoFormatNoWarn(t *testing.T) {
	if warns := CheckFormFieldFormat(projWithElement(&metadata.FormElement{
		Kind:     metadata.FormElementDatePicker,
		Name:     "Дата",
		DataPath: "Объект.Дата",
	})); len(warns) != 0 {
		t.Fatalf("без format предупреждений быть не должно, получили %d: %+v", len(warns), warns)
	}
}

// На неполедатном реквизите format тоже не применяется — предупреждаем, но другим
// текстом (без специфики нативного контрола даты).
func TestCheckFormFieldFormat_NonDateFieldGenericWarn(t *testing.T) {
	warns := CheckFormFieldFormat(projWithElement(&metadata.FormElement{
		Kind:   metadata.FormElementField,
		Name:   "Сумма",
		Format: "#,##0.00",
	}))
	if len(warns) != 1 {
		t.Fatalf("ожидалось 1 предупреждение для не-ПолеДаты, получили %d", len(warns))
	}
	if strings.Contains(warns[0].Message, "ISO") {
		t.Errorf("для не-ПолеДаты сообщение не должно говорить про ISO: %q", warns[0].Message)
	}
}

func TestCheckFormFieldFormat_NestedChildrenWalked(t *testing.T) {
	group := &metadata.FormElement{
		Kind: metadata.FormElementGroupBox,
		Name: "Группа",
		Children: []*metadata.FormElement{
			{Kind: metadata.FormElementDatePicker, Name: "Дата", Format: "dd.MM.yyyy"},
		},
	}
	if warns := CheckFormFieldFormat(projWithElement(group)); len(warns) != 1 {
		t.Fatalf("вложенный ПолеДаты должен обходиться, получили %d", len(warns))
	}
}
