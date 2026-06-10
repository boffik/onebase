package httpservice

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMatch(t *testing.T) {
	svc := &Service{
		Name:    "API",
		RootURL: "api",
		Templates: []URLTemplate{
			{Template: "/", Methods: map[string]string{"GET": "Корень"}},
			{Template: "/orders/{id}", Methods: map[string]string{"GET": "Заказ"}},
			{Template: "/orders/{id}/items", Methods: map[string]string{"GET": "Позиции"}},
			{Template: "/files/{*path}", Methods: map[string]string{"GET": "Файл"}},
		},
	}
	svc.Normalize()

	cases := []struct {
		path       string
		wantTmpl   string
		wantParams map[string]string
		wantOK     bool
	}{
		{"/", "/", map[string]string{}, true},
		{"/orders/42", "/orders/{id}", map[string]string{"id": "42"}, true},
		{"/orders/42/items", "/orders/{id}/items", map[string]string{"id": "42"}, true},
		{"/files/a/b/c.txt", "/files/{*path}", map[string]string{"path": "a/b/c.txt"}, true},
		{"/orders", "", nil, false},          // нет шаблона /orders
		{"/orders/42/extra", "", nil, false}, // лишний сегмент
	}
	for _, c := range cases {
		tmpl, params, ok := svc.Match(c.path)
		if ok != c.wantOK {
			t.Errorf("Match(%q) ok=%v, want %v", c.path, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if tmpl.Template != c.wantTmpl {
			t.Errorf("Match(%q) tmpl=%q, want %q", c.path, tmpl.Template, c.wantTmpl)
		}
		if !reflect.DeepEqual(params, c.wantParams) {
			t.Errorf("Match(%q) params=%v, want %v", c.path, params, c.wantParams)
		}
	}
}

func TestLoadDir_Normalizes(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: ЗаказыAPI
title: Заказы
root_url: /orders/
templates:
  - template: "{id}"
    methods:
      get: Получить
      post: Создать
`
	if err := os.WriteFile(filepath.Join(dir, "orders.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	services, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 {
		t.Fatalf("want 1 service, got %d", len(services))
	}
	svc := services[0]
	if svc.RootURL != "orders" {
		t.Errorf("RootURL=%q, want %q", svc.RootURL, "orders")
	}
	if svc.Auth != "none" {
		t.Errorf("Auth=%q, want default none", svc.Auth)
	}
	if got := svc.Templates[0].Template; got != "/{id}" {
		t.Errorf("template normalized=%q, want /{id}", got)
	}
	if _, ok := svc.Templates[0].Methods["GET"]; !ok {
		t.Errorf("method GET not uppercased: %v", svc.Templates[0].Methods)
	}
	if _, ok := svc.Templates[0].Methods["POST"]; !ok {
		t.Errorf("method POST not uppercased: %v", svc.Templates[0].Methods)
	}
}

func TestLoadDir_Missing(t *testing.T) {
	services, err := LoadDir(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("missing dir should be nil error, got %v", err)
	}
	if services != nil {
		t.Errorf("want nil services, got %v", services)
	}
}
