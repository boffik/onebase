package interpreter_test

import (
	"fmt"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSender implements interpreter.EmailSender for testing.
type stubSender struct {
	to      string
	subject string
	text    string
	html    string
	calls   int
}

func (s *stubSender) Configured() bool { return true }
func (s *stubSender) Send(to, subject, textBody, htmlBody string) error {
	s.to = to
	s.subject = subject
	s.text = textBody
	s.html = htmlBody
	s.calls++
	return nil
}

func TestEmailShorthand(t *testing.T) {
	stub := &stubSender{}
	src := `Процедура Тест()
  ОтправитьПисьмо("client@example.com", "Заказ принят", "Привет!");
КонецПроцедуры`
	runHTTPSrc(t, src, interpreter.NewEmailFunctions(stub, nil))
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, "client@example.com", stub.to)
	assert.Equal(t, "Заказ принят", stub.subject)
	assert.Equal(t, "Привет!", stub.text)
}

func TestEmailObject(t *testing.T) {
	stub := &stubSender{}
	src := `Процедура Тест()
  Письмо = Новый ПисьмоEmail;
  Письмо.Кому     = "boss@company.ru";
  Письмо.Тема     = "Отчёт";
  Письмо.Текст    = "Итоги за месяц";
  Письмо.HTMLТело = "<b>Итоги</b>";
  Письмо.Отправить();
КонецПроцедуры`
	runHTTPSrc(t, src, interpreter.NewEmailFunctions(stub, nil))
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, "boss@company.ru", stub.to)
	assert.Equal(t, "Отчёт", stub.subject)
	assert.Equal(t, "Итоги за месяц", stub.text)
	assert.Equal(t, "<b>Итоги</b>", stub.html)
}

func TestEmailNotConfigured(t *testing.T) {
	src := `Процедура Тест()
  Попытка
    ОтправитьПисьмо("x@y.com", "тема", "текст");
    Возврат "no error";
  Исключение
    Возврат "caught: " + ОписаниеОшибки();
  КонецПопытки;
КонецПроцедуры`
	// nil sender → should panic with user error
	extra := interpreter.NewEmailFunctions(nil, nil)
	result := runHTTPSrc(t, src, extra)
	msg, ok := result.(string)
	require.True(t, ok, fmt.Sprintf("expected string, got %T", result))
	assert.Contains(t, msg, "caught:")
	assert.Contains(t, msg, "не настроен")
}
