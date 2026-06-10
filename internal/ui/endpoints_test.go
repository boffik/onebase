package ui

// Тесты входящих REST-эндпоинтов /api/hooks/* (план 58).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/parser"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/storage"
)

// newEndpointServer поднимает Server с одним эндпоинтом и его обработчиком.
func newEndpointServer(t *testing.T, ep *metadata.Endpoint, dslSrc string) (*Server, http.Handler) {
	t.Helper()
	cat := &metadata.Entity{
		Name: "Заявка", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Комментарий", Type: metadata.FieldTypeString}},
	}
	s, _ := newSubmitTestServer(t, []*metadata.Entity{cat})

	prog, err := parser.New(lexer.New(dslSrc, ep.Handler+".endpoint.os")).ParseProgram()
	if err != nil {
		t.Fatalf("parse handler: %v", err)
	}
	s.reg.LoadEndpoints([]*metadata.Endpoint{ep}, map[string]*ast.Program{ep.Handler: prog})

	r := chi.NewRouter()
	s.MountEndpoints(r)
	return s, r
}

const echoHandlerSrc = `Процедура Обработать(Запрос, Ответ) Экспорт
  Ответ.УстановитьКод(201);
  Ответ.УстановитьЗаголовок("X-Custom", "да");
  Ответ.Тело = "{""метод"": """ + Запрос.Метод + """, ""вход"": """ + Запрос.Заголовок("X-In") + """}";
КонецПроцедуры`

// hmacHex — hex(HMAC-SHA256(body, secret)) для теста подписи.
func hmacHex(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestEndpoint_CallRunsDSLHandler(t *testing.T) {
	ep := &metadata.Endpoint{Name: "Тест", Path: "/hooks/test", Method: "POST", Auth: "none", Handler: "тест"}
	_, h := newEndpointServer(t, ep, echoHandlerSrc)

	req := httptest.NewRequest("POST", "/api/hooks/test", strings.NewReader(`{"a":1}`))
	req.Header.Set("X-In", "значение")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 201 {
		t.Fatalf("ожидался 201 от обработчика, получен %d (%s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Custom"); got != "да" {
		t.Fatalf("заголовок X-Custom: %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"метод": "POST"`) || !strings.Contains(body, `"вход": "значение"`) {
		t.Fatalf("тело: %s", body)
	}
}

func TestEndpoint_UnknownPath404(t *testing.T) {
	ep := &metadata.Endpoint{Name: "Тест", Path: "/hooks/test", Method: "POST", Auth: "none", Handler: "тест"}
	_, h := newEndpointServer(t, ep, echoHandlerSrc)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/hooks/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидался 404, получен %d", rec.Code)
	}
}

func TestEndpoint_MethodMismatch405(t *testing.T) {
	ep := &metadata.Endpoint{Name: "Тест", Path: "/hooks/test", Method: "POST", Auth: "none", Handler: "тест"}
	_, h := newEndpointServer(t, ep, echoHandlerSrc)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/hooks/test", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("ожидался 405, получен %d", rec.Code)
	}
}

func TestEndpoint_TokenAuth(t *testing.T) {
	ep := &metadata.Endpoint{Name: "Тест", Path: "/hooks/test", Method: "POST", Auth: "token", Secret: "s3cret", Handler: "тест"}
	_, h := newEndpointServer(t, ep, echoHandlerSrc)

	// без токена → 401
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/hooks/test", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("без токена ожидался 401, получен %d", rec.Code)
	}

	// неверный токен → 401
	req := httptest.NewRequest("POST", "/api/hooks/test", nil)
	req.Header.Set("X-Webhook-Token", "wrong")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("с неверным токеном ожидался 401, получен %d", rec.Code)
	}

	// верный токен → обработчик
	req = httptest.NewRequest("POST", "/api/hooks/test", nil)
	req.Header.Set("X-Webhook-Token", "s3cret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("с верным токеном ожидался 201, получен %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestEndpoint_HMACAuth(t *testing.T) {
	ep := &metadata.Endpoint{Name: "Тест", Path: "/hooks/test", Method: "POST", Auth: "hmac", Secret: "s3cret", Handler: "тест"}
	_, h := newEndpointServer(t, ep, echoHandlerSrc)

	body := `{"x":1}`

	// без подписи → 401
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/hooks/test", strings.NewReader(body)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("без подписи ожидался 401, получен %d", rec.Code)
	}

	// верная подпись → обработчик
	req := httptest.NewRequest("POST", "/api/hooks/test", strings.NewReader(body))
	req.Header.Set("X-Webhook-Signature", hmacHex("s3cret", body))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("с верной подписью ожидался 201, получен %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestEndpoint_RateLimit(t *testing.T) {
	ep := &metadata.Endpoint{Name: "Тест", Path: "/hooks/test", Method: "POST", Auth: "none", Handler: "тест", RateLimit: 2}
	_, h := newEndpointServer(t, ep, echoHandlerSrc)

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/hooks/test", nil))
		if rec.Code != 201 {
			t.Fatalf("запрос %d: ожидался 201, получен %d", i+1, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/hooks/test", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3-й запрос: ожидался 429, получен %d", rec.Code)
	}
}

// Обработчик создаёт элемент справочника обычными средствами DSL.
func TestEndpoint_HandlerWritesData(t *testing.T) {
	src := `Процедура Обработать(Запрос, Ответ) Экспорт
  Эл = Справочники.Заявка.Создать();
  Эл.Комментарий = Запрос.Параметр("текст");
  Эл.Записать();
  Ответ.УстановитьКод(200);
  Ответ.Тело = "ok";
КонецПроцедуры`
	ep := &metadata.Endpoint{Name: "Приём", Path: "/hooks/in", Method: "POST", Auth: "none", Handler: "приём"}
	s, h := newEndpointServer(t, ep, src)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/hooks/in?текст=привет", nil))
	if rec.Code != 200 {
		t.Fatalf("ожидался 200, получен %d (%s)", rec.Code, rec.Body.String())
	}

	ent := s.reg.GetEntity("Заявка")
	rows, err := s.store.List(httptest.NewRequest("GET", "/", nil).Context(), "Заявка", ent, storage.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("ожидалась 1 запись, получено %d", len(rows))
	}
	if got := asString(rows[0]["Комментарий"]); got != "привет" {
		t.Fatalf("Комментарий = %q", got)
	}
}
