package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStaticAssetsPublicWhenUsersExist: app-JS и вендор-ассеты доступны БЕЗ
// аутентификации, даже когда в базе есть пользователи и cookie сессии нет
// (план 111, P1-2). Раньше они монтировались под auth-мидлварой, поэтому каждая
// ревалидация app-JS (no-cache → 304) и каждый чанк Monaco/ECharts проходили
// через сессионную авторизацию. newServerWithUser (см. pwa_public_test.go)
// создаёт пользователя, так что auth активна — и до правки эти пути отдавали 401.
func TestStaticAssetsPublicWhenUsersExist(t *testing.T) {
	srv := newServerWithUser(t)

	routes := []string{
		"/static/ui.js",
		"/static/managed.js",
		"/static/query-builder.js",
		"/vendor/echarts/echarts.min.js",
	}
	for _, path := range routes {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("неаутентифицированный GET %s = %d, ожидался 200 (ассет должен быть публичным)", path, rec.Code)
			}
		})
	}
}
