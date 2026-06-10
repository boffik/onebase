package metadata

// Тесты модели входящих REST-эндпоинтов (план 58).

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEndpointYAML(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ep.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadEndpointFile(t *testing.T) {
	p := writeEndpointYAML(t, `
name: TelegramВходящее
path: /hooks/telegram
method: post
auth: token
secret: "s3cret"
handler: telegram_in
rate_limit: 60
`)
	ep, err := LoadEndpointFile(p)
	if err != nil {
		t.Fatalf("LoadEndpointFile: %v", err)
	}
	if ep.Name != "TelegramВходящее" || ep.Path != "/hooks/telegram" {
		t.Fatalf("распарсено: %+v", ep)
	}
	if ep.Method != "POST" {
		t.Fatalf("метод должен нормализоваться к верхнему регистру, получено %q", ep.Method)
	}
	if ep.Auth != "token" || ep.Secret != "s3cret" || ep.Handler != "telegram_in" || ep.RateLimit != 60 {
		t.Fatalf("поля: %+v", ep)
	}
}

func TestEndpointDefaults(t *testing.T) {
	p := writeEndpointYAML(t, `
name: Простой
path: hooks/simple
`)
	ep, err := LoadEndpointFile(p)
	if err != nil {
		t.Fatalf("LoadEndpointFile: %v", err)
	}
	if ep.Method != "POST" {
		t.Fatalf("метод по умолчанию POST, получено %q", ep.Method)
	}
	if ep.Auth != "none" {
		t.Fatalf("auth по умолчанию none, получено %q", ep.Auth)
	}
	if ep.Path != "/hooks/simple" {
		t.Fatalf("path должен нормализоваться с ведущим /, получено %q", ep.Path)
	}
	if ep.Handler != "простой" {
		t.Fatalf("handler по умолчанию — имя эндпоинта в нижнем регистре, получено %q", ep.Handler)
	}
}

func TestEndpointValidate(t *testing.T) {
	bad := []Endpoint{
		{Name: "x"},                                                  // нет path
		{Name: "x", Path: "/h", Auth: "странный"},                    // неизвестный auth
		{Name: "x", Path: "/h", Auth: "token"},                       // token без секрета
		{Name: "x", Path: "/h", Auth: "hmac"},                        // hmac без секрета
		{Name: "x", Path: "/h", Auth: "none", Method: "TRACE"},       // неподдерживаемый метод
		{Path: "/h", Auth: "none"},                                   // нет имени
	}
	for i, ep := range bad {
		if err := ep.Validate(); err == nil {
			t.Errorf("случай %d: ожидалась ошибка валидации (%+v)", i, ep)
		}
	}

	ok := Endpoint{Name: "x", Path: "/h", Method: "POST", Auth: "token", Secret: "s", Handler: "x"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("валидный эндпоинт не прошёл: %v", err)
	}
}
