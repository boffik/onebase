package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// anthropicServer возвращает тестовый сервер Anthropic-формата с заданным статусом.
// При 200 отдаёт корректный ответ Messages API с текстом reply.
func anthropicServer(t *testing.T, status int, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("неожиданный путь %s", r.URL.Path)
		}
		if status != 200 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"rate limit"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"` + reply + `"}],"usage":{"input_tokens":10,"output_tokens":5}}`))
	}))
}

func TestRunFallbackOn429(t *testing.T) {
	primary := anthropicServer(t, 429, "")
	defer primary.Close()
	backup := anthropicServer(t, 200, "ответ-резерва")
	defer backup.Close()

	cfg := Config{
		Enabled: true,
		Endpoints: []Endpoint{
			{Name: "ep1", Kind: KindAnthropic, BaseURL: primary.URL, APIKey: "k1"},
			{Name: "ep2", Kind: KindAnthropic, BaseURL: backup.URL, APIKey: "k2"},
		},
		Models: []Model{
			{Name: "m1", Endpoint: "ep1"},
			{Name: "m2", Endpoint: "ep2"},
		},
		Profiles: []Profile{{Task: TaskAnalysis, Models: []string{"m1", "m2"}}},
	}

	r := New(cfg, nil)
	resp, err := r.Run(context.Background(), TaskAnalysis, ChatRequest{Messages: []Message{UserText("привет")}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Text != "ответ-резерва" {
		t.Fatalf("ожидался ответ резервной модели, получено %q", resp.Text)
	}
	if resp.Model != "m2" {
		t.Fatalf("ожидалась модель m2, получено %q", resp.Model)
	}
}

func TestRunNoFallbackOn400(t *testing.T) {
	bad := anthropicServer(t, 400, "")
	defer bad.Close()
	cfg := Config{
		Enabled:   true,
		Endpoints: []Endpoint{{Name: "ep", Kind: KindAnthropic, BaseURL: bad.URL, APIKey: "k"}},
		Models:    []Model{{Name: "m", Endpoint: "ep"}},
		Profiles:  []Profile{{Task: TaskAnalysis, Models: []string{"m"}}},
	}
	r := New(cfg, nil)
	_, err := r.Run(context.Background(), TaskAnalysis, ChatRequest{Messages: []Message{UserText("x")}})
	if err == nil {
		t.Fatal("ожидалась ошибка на 400 без фолбэка")
	}
}

func TestRunVisionSkipsNonVision(t *testing.T) {
	// Первая модель не vision — при картинке должна быть пропущена, ответит vision-модель.
	textOnly := anthropicServer(t, 200, "текстовая")
	defer textOnly.Close()
	visionSrv := anthropicServer(t, 200, "распознано")
	defer visionSrv.Close()

	cfg := Config{
		Enabled: true,
		Endpoints: []Endpoint{
			{Name: "t", Kind: KindAnthropic, BaseURL: textOnly.URL, APIKey: "k"},
			{Name: "v", Kind: KindAnthropic, BaseURL: visionSrv.URL, APIKey: "k"},
		},
		Models: []Model{
			{Name: "text", Endpoint: "t"},
			{Name: "vis", Endpoint: "v", Vision: true},
		},
		Profiles: []Profile{{Task: TaskDocuments, Models: []string{"text", "vis"}}},
	}
	r := New(cfg, nil)
	req := ChatRequest{Messages: []Message{{Role: RoleUser, Parts: []Part{ImagePart("AAAA", "image/png")}}}}
	resp, err := r.Run(context.Background(), TaskDocuments, req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Model != "vis" {
		t.Fatalf("ожидалась vision-модель vis, получено %q", resp.Model)
	}
}

func TestResolveDisabled(t *testing.T) {
	_, err := Config{Enabled: false}.Resolve(TaskAnalysis)
	if err == nil {
		t.Fatal("ожидалась ошибка для выключенного конфига")
	}
}

func TestConfigRedactsKeys(t *testing.T) {
	cfg := Config{Endpoints: []Endpoint{{Name: "e", APIKey: "supersecretkey"}}}
	red := cfg.Redacted()
	if red.Endpoints[0].APIKey == "supersecretkey" {
		t.Fatal("ключ не замаскирован")
	}
	if cfg.Endpoints[0].APIKey != "supersecretkey" {
		t.Fatal("Redacted мутировал оригинал")
	}
}
