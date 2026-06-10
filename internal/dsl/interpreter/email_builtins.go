package interpreter

import "fmt"

// EmailSender is the minimal interface required by email DSL functions.
type EmailSender interface {
	Send(to, subject, textBody, htmlBody string) error
	Configured() bool
}

// ─── dslEmail (Новый ПисьмоEmail) ────────────────────────────────────────────

type dslEmail struct {
	sender  EmailSender
	guard   NetGuard
	to      string
	cc      string
	subject string
	text    string
	html    string
}

func (e *dslEmail) Get(field string) any {
	switch field {
	case "кому", "to":
		return e.to
	case "копия", "cc":
		return e.cc
	case "тема", "subject":
		return e.subject
	case "текст", "text", "body":
		return e.text
	case "htmlтело", "htmlbody":
		return e.html
	}
	return nil
}

func (e *dslEmail) Set(field string, val any) {
	s := fmt.Sprintf("%v", val)
	switch field {
	case "кому", "to":
		e.to = s
	case "копия", "cc":
		e.cc = s
	case "тема", "subject":
		e.subject = s
	case "текст", "text", "body":
		e.text = s
	case "htmlтело", "htmlbody":
		e.html = s
	}
}

func (e *dslEmail) CallMethod(name string, args []any) any {
	switch name {
	case "отправить", "send":
		checkNet(e.guard)
		if e.to == "" {
			panic(userError{Msg: "ПисьмоEmail.Отправить: поле Кому не задано"})
		}
		if e.subject == "" {
			panic(userError{Msg: "ПисьмоEmail.Отправить: поле Тема не задана"})
		}
		if err := e.sender.Send(e.to, e.subject, e.text, e.html); err != nil {
			panic(userError{Msg: "ОтправитьПисьмо: " + err.Error()})
		}
		return nil
	}
	panic(userError{Msg: "ПисьмоEmail: неизвестный метод " + name})
}

// ─── NewEmailFunctions ────────────────────────────────────────────────────────

// NewEmailFunctions returns DSL functions/factories to inject into extraVars.
// If sender is nil or not configured, functions panic with a user-friendly message.
func NewEmailFunctions(sender EmailSender, guard NetGuard) map[string]any {
	send := func(to, subject, textBody string) {
		checkNet(guard)
		if sender == nil || !sender.Configured() {
			panic(userError{Msg: "email не настроен — добавьте секцию email: в config/app.yaml"})
		}
		if err := sender.Send(to, subject, textBody, ""); err != nil {
			panic(userError{Msg: "ОтправитьПисьмо: " + err.Error()})
		}
	}

	shorthand := BuiltinFunc(func(args []any, file string, line int) (any, error) {
		to := strArg(args, 0)
		subject := strArg(args, 1)
		text := strArg(args, 2)
		send(to, subject, text)
		return nil, nil
	})

	emailFactory := func(args []any) any {
		checkNet(guard)
		if sender == nil || !sender.Configured() {
			panic(userError{Msg: "email не настроен — добавьте секцию email: в config/app.yaml"})
		}
		return &dslEmail{sender: sender, guard: guard}
	}

	return map[string]any{
		"ОтправитьПисьмо":     shorthand,
		"SendEmail":            shorthand,
		"__factory_ПисьмоEmail": emailFactory,
		"__factory_EmailMessage": emailFactory,
	}
}
