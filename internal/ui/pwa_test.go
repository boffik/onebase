package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestMountPWA проверяет, что маршруты PWA отдают корректные Content-Type и
// заголовки кэширования, а /sw.js — из корня (scope «/»).
func TestMountPWA(t *testing.T) {
	r := chi.NewRouter()
	mountPWA(r)

	cases := []struct {
		path        string
		wantCT      string // подстрока в Content-Type
		wantCache   string // подстрока в Cache-Control
		wantNonZero bool
	}{
		{"/manifest.webmanifest", "application/manifest+json", "max-age=3600", true},
		{"/sw.js", "javascript", "no-cache", true},
		{"/offline.html", "text/html", "no-cache", true},
		{"/icons/icon-192.png", "image/png", "immutable", true},
		{"/icons/icon-512.png", "image/png", "immutable", true},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, c.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("статус = %d, ожидался 200", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, c.wantCT) {
				t.Errorf("Content-Type = %q, ожидалась подстрока %q", ct, c.wantCT)
			}
			if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, c.wantCache) {
				t.Errorf("Cache-Control = %q, ожидалась подстрока %q", cc, c.wantCache)
			}
			if c.wantNonZero && rec.Body.Len() == 0 {
				t.Errorf("пустое тело ответа")
			}
		})
	}
}

// TestServiceWorkerCacheVersioned проверяет, что при отдаче /sw.js placeholder
// имени кэша подставлен ревизией сборки — иначе авто-инвалидация при релизе не
// работает (ревью PR #34: ручной CACHE-bump → залипание vendor-ассетов).
func TestServiceWorkerCacheVersioned(t *testing.T) {
	r := chi.NewRouter()
	mountPWA(r)

	req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "__OB_CACHE__") {
		t.Error("в отданном /sw.js остался placeholder __OB_CACHE__ — версия не подставлена")
	}
	if !strings.Contains(body, "onebase-") {
		t.Error("имя кэша должно содержать префикс onebase-")
	}
	if !strings.Contains(body, "addEventListener('fetch'") {
		t.Error("в /sw.js нет обработчика fetch — отдан не тот файл")
	}
}

// TestHeadHasPWATags — smoke-тест из плана 45: рендер шаблона head содержит
// viewport, ссылку на manifest и регистрацию service worker. Защищает от
// случайного удаления этих тегов при правке tplHead.
func TestHeadHasPWATags(t *testing.T) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "head", map[string]any{"Cfg": Config{}}); err != nil {
		t.Fatalf("рендер head: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`name="viewport"`, `rel="manifest"`, "/sw.js"} {
		if !strings.Contains(out, want) {
			t.Errorf("head не содержит %q", want)
		}
	}
}

// TestManifestValid проверяет, что манифест — валидный JSON с обязательными
// полями PWA.
func TestManifestValid(t *testing.T) {
	data, err := pwaFS.ReadFile("pwa/manifest.webmanifest")
	if err != nil {
		t.Fatalf("чтение манифеста: %v", err)
	}
	var m struct {
		Name    string `json:"name"`
		StartU  string `json:"start_url"`
		Display string `json:"display"`
		Icons   []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("манифест не парсится как JSON: %v", err)
	}
	if m.Name == "" {
		t.Error("manifest.name пуст")
	}
	if m.StartU == "" {
		t.Error("manifest.start_url пуст")
	}
	if m.Display == "" {
		t.Error("manifest.display пуст")
	}
	if len(m.Icons) == 0 {
		t.Error("manifest.icons пуст")
	}
}
