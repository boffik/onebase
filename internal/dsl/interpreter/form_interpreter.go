package interpreter

import (
	"context"
	"fmt"
	"strings"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/metadata"
)

// FormContext provides context for form module execution
type FormContext struct {
	EntityName   string                    // entity name
	FormName     string                    // form name
	Object       This                      // current object (ref)
	Mode         string                    // form mode: "view", "edit", "choice"
	SelectedItem This                      // selected item (for choice forms)
	Parameters   map[string]any            // form parameters
	Elements     map[string]*FormElementProxy // form elements access
}

// FormElementProxy provides access to form element properties
type FormElementProxy struct {
	Name    string
	Visible *bool
	Enabled *bool
	Value   any
	Title   string
	Props   map[string]any
}

func (p *FormElementProxy) Get(field string) any {
	switch strings.ToLower(field) {
	case "имя", "name":
		return p.Name
	case "видимость", "visible":
		if p.Visible != nil {
			return *p.Visible
		}
		return true
	case "доступность", "enabled":
		if p.Enabled != nil {
			return *p.Enabled
		}
		return true
	case "значение", "value":
		return p.Value
	case "заголовок", "title":
		return p.Title
	default:
		if p.Props != nil {
			return p.Props[field]
		}
		return nil
	}
}

func (p *FormElementProxy) Set(field string, val any) {
	switch strings.ToLower(field) {
	case "видимость", "visible":
		b := truthy(val)
		p.Visible = &b
	case "доступность", "enabled":
		b := truthy(val)
		p.Enabled = &b
	case "значение", "value":
		p.Value = val
	case "заголовок", "title":
		p.Title = fmt.Sprintf("%v", val)
	default:
		if p.Props == nil {
			p.Props = make(map[string]any)
		}
		p.Props[field] = val
	}
}

// FormInterpreter handles form module execution
type FormInterpreter struct {
	base      *Interpreter
	lookupMod func(name string) *ast.ProcedureDecl
}

// NewFormInterpreter creates form interpreter
func NewFormInterpreter(base *Interpreter, lookupMod func(name string) *ast.ProcedureDecl) *FormInterpreter {
	return &FormInterpreter{
		base:      base,
		lookupMod: lookupMod,
	}
}

// ExecuteEventHandler executes form event handler
func (fi *FormInterpreter) ExecuteEventHandler(ctx context.Context, form *metadata.FormModule, eventName string, formCtx *FormContext) error {
	// Find handler procedure
	var procName string

	// Check form-level handlers
	if form.Handlers != nil {
		if eventType := metadata.FormEventType(eventName); eventType != "" {
			if handler, ok := form.Handlers[eventType]; ok {
				procName = handler
			}
		}
	}

	if procName == "" {
		// Try to find procedure by event name directly (1C style)
		procName = eventName
	}

	// Look up procedure
	proc := fi.findProcedure(form, procName)
	if proc == nil {
		return nil // no handler - not an error
	}

	return fi.ExecuteProcedure(ctx, proc, formCtx)
}

// ExecuteElementEvent executes element event handler
func (fi *FormInterpreter) ExecuteElementEvent(ctx context.Context, form *metadata.FormModule, elementName, eventName string, formCtx *FormContext) error {
	element := form.GetElementByName(elementName)
	if element == nil {
		return fmt.Errorf("element not found: %s", elementName)
	}

	if element.Handlers == nil {
		return nil
	}

	eventType := metadata.FormEventType(eventName)
	if eventType == "" {
		return nil
	}

	procName, ok := element.Handlers[eventType]
	if !ok {
		return nil
	}

	proc := fi.findProcedure(form, procName)
	if proc == nil {
		return nil
	}

	return fi.ExecuteProcedure(ctx, proc, formCtx)
}

// ExecuteProcedure executes form procedure with parameters
func (fi *FormInterpreter) ExecuteProcedure(ctx context.Context, proc *metadata.FormProcedure, formCtx *FormContext) error {
	// Parse procedure body
	program, err := fi.parseFormProcedure(proc)
	if err != nil {
		return err
	}

	if len(program.Procedures) == 0 {
		return nil
	}

	decl := program.Procedures[0]

	// Create execution environment
	this := formCtx.Object
	extraVars := map[string]any{
		"ЭтотОбъект": this,
		"ЭтотОбъектЭлемент": nil, // will be set in element context
	}

	// Add form elements access
	if formCtx.Elements == nil {
		formCtx.Elements = make(map[string]*FormElementProxy)
	}
	extraVars["ЭлементыФормы"] = &FormElementsProxy{
		elements: formCtx.Elements,
	}

	// Add form context
	extraVars["Режим"] = formCtx.Mode
	extraVars["Параметры"] = goMapToMap(formCtx.Parameters)

	return fi.base.Run(decl, this, extraVars)
}

// parseFormProcedure parses form procedure DSL source
func (fi *FormInterpreter) parseFormProcedure(proc *metadata.FormProcedure) (*ast.Program, error) {
	// For now, we'll use the existing parser
	// In production, this would cache parsed procedures
	return nil, nil
}

func (fi *FormInterpreter) findProcedure(form *metadata.FormModule, name string) *metadata.FormProcedure {
	if form.Procedures == nil {
		return nil
	}

	// Case-insensitive lookup
	lowerName := strings.ToLower(name)
	for _, proc := range form.Procedures {
		if strings.ToLower(proc.Name) == lowerName {
			return proc
		}
	}
	return nil
}

// FormElementsProxy provides access to form elements
type FormElementsProxy struct {
	elements map[string]*FormElementProxy
}

func (p *FormElementsProxy) Get(key string) any {
	if el, ok := p.elements[key]; ok {
		return el
	}
	return nil
}

func (p *FormElementsProxy) Set(key string, val any) {
	// Elements are added by form initialization, not set dynamically
}

// FormEventContext holds information about form event
type FormEventContext struct {
	EventType    string
	ElementName  string
	Parameters   map[string]any
	Cancel       bool
	StandardProcessing bool
}

// ExecuteOnOpen executes ПриОткрытии (OnOpen) handler
func (fi *FormInterpreter) ExecuteOnOpen(ctx context.Context, form *metadata.FormModule, formCtx *FormContext) error {
	return fi.ExecuteEventHandler(ctx, form, "ПриОткрытии", formCtx)
}

// ExecuteBeforeWrite executes ПередЗаписью (BeforeWrite) handler
func (fi *FormInterpreter) ExecuteBeforeWrite(ctx context.Context, form *metadata.FormModule, formCtx *FormContext, writeParams map[string]any) error {
	if writeParams != nil {
		if formCtx.Parameters == nil {
			formCtx.Parameters = make(map[string]any)
		}
		for k, v := range writeParams {
			formCtx.Parameters[k] = v
		}
	}
	return fi.ExecuteEventHandler(ctx, form, "ПередЗаписью", formCtx)
}

// ExecuteOnWrite executes ПриЗаписи (OnWrite) handler
func (fi *FormInterpreter) ExecuteOnWrite(ctx context.Context, form *metadata.FormModule, formCtx *FormContext) error {
	return fi.ExecuteEventHandler(ctx, form, "ПриЗаписи", formCtx)
}

// ExecuteAfterWrite executes ПослеЗаписи (AfterWrite) handler
func (fi *FormInterpreter) ExecuteAfterWrite(ctx context.Context, form *metadata.FormModule, formCtx *FormContext) error {
	return fi.ExecuteEventHandler(ctx, form, "ПослеЗаписи", formCtx)
}

// ExecuteBeforeClose executes ПередЗакрытием (BeforeClose) handler
func (fi *FormInterpreter) ExecuteBeforeClose(ctx context.Context, form *metadata.FormModule, formCtx *FormContext) (bool, error) {
	// Returns (cancel, error)
	err := fi.ExecuteEventHandler(ctx, form, "ПередЗакрытием", formCtx)
	if err != nil {
		return false, err
	}
	// Check if cancellation was requested via context
	if formCtx.Parameters != nil {
		if cancel, ok := formCtx.Parameters["Отказ"]; ok && truthy(cancel) {
			return true, nil
		}
	}
	return false, nil
}

// ExecuteOnChange executes ПриИзменении (OnChange) handler for element
func (fi *FormInterpreter) ExecuteOnChange(ctx context.Context, form *metadata.FormModule, elementName string, formCtx *FormContext) error {
	return fi.ExecuteElementEvent(ctx, form, elementName, "ПриИзменении", formCtx)
}

// События табличных частей (замечание #15). FormInterpreter уже умеет
// диспатчить произвольные события через ExecuteElementEvent — эти методы
// дают удобный API для UI-обработчиков, которые знают тип события заранее.
// Тригерринг из браузера реализуется отдельно в UI/JS — здесь только
// серверная сторона исполнения.

// ExecuteOnRowAdded executes ПриДобавленииСтроки handler for a table part.
func (fi *FormInterpreter) ExecuteOnRowAdded(ctx context.Context, form *metadata.FormModule, tablePartName string, formCtx *FormContext) error {
	return fi.ExecuteElementEvent(ctx, form, tablePartName, string(metadata.FormEventOnRowAdded), formCtx)
}

// ExecuteOnRowChanged executes ПриИзмененииСтроки handler for a table part.
func (fi *FormInterpreter) ExecuteOnRowChanged(ctx context.Context, form *metadata.FormModule, tablePartName string, formCtx *FormContext) error {
	return fi.ExecuteElementEvent(ctx, form, tablePartName, string(metadata.FormEventOnRowChanged), formCtx)
}

// ExecuteOnRowDeleted executes ПриУдаленииСтроки handler for a table part.
func (fi *FormInterpreter) ExecuteOnRowDeleted(ctx context.Context, form *metadata.FormModule, tablePartName string, formCtx *FormContext) error {
	return fi.ExecuteElementEvent(ctx, form, tablePartName, string(metadata.FormEventOnRowDeleted), formCtx)
}

// ExecuteOnRowActivated executes ПриАктивизацииСтроки handler for a table part.
func (fi *FormInterpreter) ExecuteOnRowActivated(ctx context.Context, form *metadata.FormModule, tablePartName string, formCtx *FormContext) error {
	return fi.ExecuteElementEvent(ctx, form, tablePartName, string(metadata.FormEventOnRowActivated), formCtx)
}

// ExecuteItemChoice executes ОбработкаВыбора (ItemChoice) handler
func (fi *FormInterpreter) ExecuteItemChoice(ctx context.Context, form *metadata.FormModule, elementName string, selectedItem This, formCtx *FormContext) error {
	formCtx.SelectedItem = selectedItem
	return fi.ExecuteElementEvent(ctx, form, elementName, "ОбработкаВыбора", formCtx)
}

// ExecuteStartChoice executes НачалоВыбора (StartChoice) handler
func (fi *FormInterpreter) ExecuteStartChoice(ctx context.Context, form *metadata.FormModule, elementName string, formCtx *FormContext) error {
	return fi.ExecuteElementEvent(ctx, form, elementName, "НачалоВыбора", formCtx)
}

// ExecuteOnCreate executes ПриСоздании (OnCreate) handler
func (fi *FormInterpreter) ExecuteOnCreate(ctx context.Context, form *metadata.FormModule, formCtx *FormContext) error {
	return fi.ExecuteEventHandler(ctx, form, "ПриСоздании", formCtx)
}

// BuildFormContext creates FormContext from runtime values
func BuildFormContext(entityName, formName string, object This, mode string, params map[string]any) *FormContext {
	return &FormContext{
		EntityName: entityName,
		FormName:   formName,
		Object:     object,
		Mode:       mode,
		Parameters: params,
		Elements:   make(map[string]*FormElementProxy),
	}
}

// goMapToMap converts a Go map[string]any to a DSL Map
func goMapToMap(m map[string]any) *Map {
	result := &Map{}
	if m == nil {
		return result
	}
	for k, v := range m {
		result.keys = append(result.keys, k)
		result.vals = append(result.vals, v)
	}
	return result
}
