package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRegistry_RecordsAndExposes(t *testing.T) {
	reg := New()

	r := chi.NewRouter()
	r.Use(reg.Middleware)
	r.Get("/documents/{entity}/{id}/post", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {})

	// Два запроса на проведение и один на health.
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/documents/Счёт/123/post", nil))
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var sb strings.Builder
	reg.WritePrometheus(&sb)
	out := sb.String()

	// Метка route — это шаблон chi, а не конкретный id (низкая кардинальность).
	if !strings.Contains(out, `onebase_http_requests_total{method="GET",route="/documents/{entity}/{id}/post",status="201"} 2`) {
		t.Errorf("ожидали счётчик проведения =2 с шаблонной меткой route, получили:\n%s", out)
	}
	if !strings.Contains(out, `route="/health",status="200"} 1`) {
		t.Errorf("ожидали счётчик health=1, получили:\n%s", out)
	}
	// Гистограмма: count по маршруту проведения == 2, есть +Inf корзина.
	if !strings.Contains(out, `onebase_http_request_duration_seconds_count{method="GET",route="/documents/{entity}/{id}/post"} 2`) {
		t.Errorf("ожидали histogram count=2, получили:\n%s", out)
	}
	if !strings.Contains(out, `le="+Inf"`) {
		t.Errorf("ожидали корзину +Inf, получили:\n%s", out)
	}
	// TYPE-строки обязательны для валидного exposition.
	if !strings.Contains(out, "# TYPE onebase_http_requests_total counter") ||
		!strings.Contains(out, "# TYPE onebase_http_request_duration_seconds histogram") {
		t.Errorf("отсутствуют TYPE-заголовки, получили:\n%s", out)
	}
}

// Незаматченный путь группируется под route="other", а не плодит серию.
func TestRegistry_UnmatchedRouteIsOther(t *testing.T) {
	reg := New()
	r := chi.NewRouter()
	r.Use(reg.Middleware)
	// Хотя бы один обычный маршрут нужен, чтобы chi построил цепочку middleware
	// (иначе при наличии только NotFound запрос идёт мимо Use). В реальном
	// сервере маршруты всегда есть.
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {})
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/no/such/path", nil))

	var sb strings.Builder
	reg.WritePrometheus(&sb)
	if !strings.Contains(sb.String(), `route="other",status="404"`) {
		t.Errorf("ожидали route=other для незаматченного пути, получили:\n%s", sb.String())
	}
}
