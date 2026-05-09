package metadata

import (
	"strings"
)

// FormEventType represents types of form events (1C-style)
type FormEventType string

const (
	FormEventOnOpen        FormEventType = "ПриОткрытии"         // OnOpen
	FormEventBeforeWrite   FormEventType = "ПередЗаписью"        // BeforeWrite
	FormEventOnWrite       FormEventType = "ПриЗаписи"           // OnWrite
	FormEventAfterWrite    FormEventType = "ПослеЗаписи"         // AfterWrite
	FormEventBeforeClose   FormEventType = "ПередЗакрытием"      // BeforeClose
	FormEventOnClose       FormEventType = "ПриЗакрытии"         // OnClose
	FormEventOnActivate    FormEventType = "ПриАктивации"        // OnActivate
	FormEventItemChoice    FormEventType = "ОбработкаВыбора"     // ItemChoice
	FormEventStartChoice   FormEventType = "НачалоВыбора"        // StartChoice
	FormEventOnChange      FormEventType = "ПриИзменении"        // OnChange
	FormEventOnCreate      FormEventType = "ПриСоздании"         // OnCreate
	FormEventBeforeDelete  FormEventType = "ПередУдалением"      // BeforeDelete
	FormEventOnDelete      FormEventType = "ПриУдалении"         // OnDelete
	FormEventAfterDelete   FormEventType = "ПослеУдаления"       // AfterDelete
)

// FormElementType represents types of form elements
type FormElementType string

const (
	FormElementField          FormElementType = "ПолеВвода"        // InputField
	FormElementLabel          FormElementType = "Надпись"          // Label
	FormElementButton         FormElementType = "Кнопка"           // Button
	FormElementTable          FormElementType = "Таблица"          // Table
	FormElementGroupBox       FormElementType = "ГруппаФормы"      // FormGroup
	FormElementPage           FormElementType = "Страница"         // Page
	FormElementPages          FormElementType = "СтраницыФормы"    // FormPages
	FormElementCheckbox       FormElementType = "Флажок"           // Checkbox
	FormElementSwitch         FormElementType = "Переключатель"    // Switch
	FormElementInputList      FormElementType = "ПолеСписка"       // InputList
	FormElementDatePicker     FormElementType = "ПолеДаты"         // DateField
	FormElementFormField      FormElementType = "ПолеФормы"        // FormField
	FormElementTablePart      FormElementType = "ТабличнаяЧасть"   // TablePart
)

// FormElement represents a single form element (field, button, etc.)
type FormElement struct {
	ID          string                    // unique identifier
	Name        string                    // element name for DSL access
	Kind        FormElementType           // element type
	Title       string                    // display title
	FieldName   string                    // bound field name (for fields)
	TablePart   string                    // table part name (for table parts)
	Visible     bool                      // is visible
	Enabled     bool                      // is enabled
	Required    bool                      // is required
	Handlers    map[FormEventType]string  // event handlers (proc name)
	Props       map[string]any            // additional properties
	Children    []*FormElement            // child elements (for groups)
}

// FormModule represents a form module with event handlers
type FormModule struct {
	EntityName  string                    // parent entity name
	Name        string                    // form name (e.g. "ФормаОбъекта", "ФормаСписка")
	Kind        string                    // form kind: "object", "list"
	Elements    []*FormElement            // form elements structure
	Handlers    map[FormEventType]string  // form-level event handlers
	Procedures  map[string]*FormProcedure // procedures defined in form module
}

// FormProcedure represents a procedure in form module
type FormProcedure struct {
	Name       string              // procedure name
	Params     []FormProcParam     // parameters
	Body       string              // procedure body (DSL source)
	IsExport   bool                // is exported
}

// FormProcParam represents a procedure parameter
type FormProcParam struct {
	Name string // parameter name
	Type string // parameter type
}

// EventHandlerInfo contains information about event handler
type EventHandlerInfo struct {
	ElementName  string           // element name (empty for form-level events)
	EventType    FormEventType    // event type
	ProcName     string           // procedure name to call
}

// GetEventHandler returns handler for element event
func (fm *FormModule) GetEventHandler(elementName string, eventType FormEventType) (string, bool) {
	// First check element handlers
	if elementName != "" {
		for _, el := range fm.Elements {
			if handler := findElementHandler(el, eventNameToID(elementName), eventType); handler != "" {
				return handler, true
			}
		}
	}
	// Then check form-level handlers
	if fm.Handlers != nil {
		if handler, ok := fm.Handlers[eventType]; ok {
			return handler, true
		}
	}
	return "", false
}

// findElementHandler recursively searches for element handler
func findElementHandler(el *FormElement, elementID string, eventType FormEventType) string {
	if el.ID == elementID {
		if el.Handlers != nil {
			if handler, ok := el.Handlers[eventType]; ok {
				return handler
			}
		}
		return ""
	}
	for _, child := range el.Children {
		if handler := findElementHandler(child, elementID, eventType); handler != "" {
			return handler
		}
	}
	return ""
}

// eventNameToID converts element name to ID format
func eventNameToID(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "_"))
}

// GetElementByName finds element by name
func (fm *FormModule) GetElementByName(name string) *FormElement {
	for _, el := range fm.Elements {
		if el := findElementByName(el, name); el != nil {
			return el
		}
	}
	return nil
}

func findElementByName(el *FormElement, name string) *FormElement {
	if el.Name == name {
		return el
	}
	for _, child := range el.Children {
		if found := findElementByName(child, name); found != nil {
			return found
		}
	}
	return nil
}

// StandardFormNames returns standard form names for 1C compatibility
func StandardFormNames() []string {
	return []string{
		"ФормаОбъекта",   // ObjectForm
		"ФормаСписка",    // ListForm
		"ФормаВыбора",    // ChoiceForm
		"ФормаГруппы",    // FolderForm (for hierarchical catalogs)
	}
}

// IsStandardForm checks if form name is standard
func IsStandardForm(name string) bool {
	for _, std := range StandardFormNames() {
		if strings.EqualFold(name, std) {
			return true
		}
	}
	return false
}

// FormModuleFileName returns .os filename for form module
func FormModuleFileName(entityName, formName string) string {
	base := strings.ToLower(entityName)
	if formName != "" && !IsStandardForm(formName) {
		return base + "_" + strings.ToLower(formName) + ".form.os"
	}
	return base + ".form.os"
}

// ObjectFormFileName returns .os filename for object form
func ObjectFormFileName(entityName string) string {
	return strings.ToLower(entityName) + ".form.os"
}
