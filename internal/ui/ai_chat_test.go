package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/llm"
)

// mockAnthropic возвращает тестовый сервер Anthropic-формата с фиксированным ответом.
func mockAnthropic(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"` + reply + `"}],"usage":{}}`))
	}))
}

func chatConfig(baseURL string) llm.Config {
	return llm.Config{
		Enabled:   true,
		Endpoints: []llm.Endpoint{{Name: "ep", Kind: llm.KindAnthropic, BaseURL: baseURL, APIKey: "k"}},
		Models:    []llm.Model{{Name: "m", Endpoint: "ep"}},
		Profiles:  []llm.Profile{{Task: "чат", Models: []string{"m"}}},
	}
}

func TestAIEnabled(t *testing.T) {
	s, ctx := newSubmitTestServer(t, nil)

	// Без конфига — выключено.
	rr := httptest.NewRecorder()
	s.aiEnabled(rr, httptest.NewRequest("GET", "/ui/ai/enabled", nil))
	var out struct{ Enabled bool }
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out.Enabled {
		t.Fatal("ожидалось enabled=false без конфига")
	}

	// После сохранения конфига с моделью — включено.
	if err := s.store.SaveLLMConfig(ctx, chatConfig("http://example")); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	s.aiEnabled(rr, httptest.NewRequest("GET", "/ui/ai/enabled", nil))
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if !out.Enabled {
		t.Fatal("ожидалось enabled=true после сохранения конфига")
	}
}

func TestAIChat(t *testing.T) {
	s, ctx := newSubmitTestServer(t, nil)
	srv := mockAnthropic(t, "Здравствуйте! Чем помочь?")
	defer srv.Close()
	if err := s.store.SaveLLMConfig(ctx, chatConfig(srv.URL)); err != nil {
		t.Fatal(err)
	}

	body := `{"messages":[{"role":"user","content":"привет"}]}`
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/ui/ai/chat", strings.NewReader(body))
	s.aiChat(rr, r)

	var out struct {
		OK    bool   `json:"ok"`
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("разбор ответа: %v (%s)", err, rr.Body.String())
	}
	if !out.OK {
		t.Fatalf("ожидался ok, получено: %s", out.Error)
	}
	if out.Text != "Здравствуйте! Чем помочь?" {
		t.Fatalf("неожиданный текст: %q", out.Text)
	}
}

func TestAIChatEmpty(t *testing.T) {
	s, _ := newSubmitTestServer(t, nil)
	rr := httptest.NewRecorder()
	s.aiChat(rr, httptest.NewRequest("POST", "/ui/ai/chat", strings.NewReader(`{"messages":[]}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ожидался 400 на пустой запрос, получено %d", rr.Code)
	}
}
