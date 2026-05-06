package interpreter_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runHTTPSrc(t *testing.T, src string, extra map[string]any) any {
	t.Helper()
	l := lexer.New(src, "test.os")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	require.NoError(t, err)
	require.NotEmpty(t, prog.Procedures)

	interp := interpreter.New()
	var result any
	err = interp.RunWithResult(prog.Procedures[0], nil, &result, extra)
	require.NoError(t, err)
	return result
}

func TestHTTPGet_Shorthand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	src := fmt.Sprintf(`Функция Тест()
  Ответ = HTTPПолучить("%s/data");
  Возврат Ответ.КодСостояния;
КонецФункции`, srv.URL)

	result := runHTTPSrc(t, src, interpreter.NewHTTPFunctions())
	assert.Equal(t, float64(200), result)
}

func TestHTTPConnection_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rates", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		fmt.Fprint(w, "hello")
	}))
	defer srv.Close()

	host := srv.Listener.Addr().String()
	src := fmt.Sprintf(`Функция Тест()
  Соединение = Новый HTTPСоединение("%s");
  Запрос = Новый HTTPЗапрос("/rates");
  Ответ = Соединение.Получить(Запрос);
  Возврат Ответ.ПолучитьТелоКакСтроку();
КонецФункции`, host)

	result := runHTTPSrc(t, src, interpreter.NewHTTPFunctions())
	assert.Equal(t, "hello", result)
}

func TestHTTPConnection_Post(t *testing.T) {
	var gotBody string
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		gotHeader = r.Header.Get("Content-Type")
		w.WriteHeader(201)
		fmt.Fprint(w, `{"created":true}`)
	}))
	defer srv.Close()

	host := srv.Listener.Addr().String()
	src := fmt.Sprintf(`Функция Тест()
  Соединение = Новый HTTPСоединение("%s");
  Запрос = Новый HTTPЗапрос("/orders");
  Запрос.УстановитьЗаголовок("Content-Type", "application/json");
  Запрос.УстановитьТелоИзСтроки("{""status"":""new""}");
  Ответ = Соединение.ОтправитьДля(Запрос, "POST");
  Возврат Ответ.КодСостояния;
КонецФункции`, host)

	result := runHTTPSrc(t, src, interpreter.NewHTTPFunctions())
	assert.Equal(t, float64(201), result)
	assert.Equal(t, `{"status":"new"}`, gotBody)
	assert.Equal(t, "application/json", gotHeader)
}

func TestHTTPResponse_GetHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "abc123")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	src := fmt.Sprintf(`Функция Тест()
  Ответ = HTTPПолучить("%s");
  Возврат Ответ.ПолучитьЗаголовок("X-Request-Id");
КонецФункции`, srv.URL)

	result := runHTTPSrc(t, src, interpreter.NewHTTPFunctions())
	assert.Equal(t, "abc123", result)
}

func TestHTTPGet_WithJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"rate":75}`)
	}))
	defer srv.Close()

	src := fmt.Sprintf(`Функция Тест()
  Ответ = HTTPПолучить("%s/v1/rates");
  Если Ответ.КодСостояния = 200 Тогда
    данные = ПрочитатьJSON(Ответ.ПолучитьТелоКакСтроку());
    Возврат данные.Получить("rate");
  КонецЕсли;
  Возврат 0;
КонецФункции`, srv.URL)

	result := runHTTPSrc(t, src, interpreter.NewHTTPFunctions())
	assert.Equal(t, int64(75), result)
}
