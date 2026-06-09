package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureAnthropic запускает тестовый сервер, захватывает тело запроса в body и
// отвечает корректным ответом Anthropic Messages API.
func captureAnthropic(t *testing.T, captured *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("неожиданный путь %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		*captured = m
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
}

// TestAnthropicJSONDirectiveAppended проверяет, что при req.JSON=true системный промпт
// содержит оригинальный текст и JSON-директиву.
func TestAnthropicJSONDirectiveAppended(t *testing.T) {
	var captured map[string]any
	srv := captureAnthropic(t, &captured)
	defer srv.Close()

	rm := ResolvedModel{
		Endpoint: Endpoint{Name: "ep", Kind: KindAnthropic, BaseURL: srv.URL, APIKey: "k"},
		Model:    Model{Name: "claude-test", Endpoint: "ep", MaxTokens: 100},
	}
	req := ChatRequest{
		System:   "Ты бухгалтер",
		JSON:     true,
		Messages: []Message{UserText("привет")},
	}

	_, err := completeAnthropic(context.Background(), http.DefaultClient, rm, req)
	if err != nil {
		t.Fatalf("completeAnthropic: %v", err)
	}

	sys, _ := captured["system"].(string)
	if !strings.Contains(sys, "Ты бухгалтер") {
		t.Errorf("system не содержит оригинальный текст; получено %q", sys)
	}
	if !strings.Contains(sys, "JSON") {
		t.Errorf("system не содержит JSON-директиву; получено %q", sys)
	}
}

// TestAnthropicTemperatureNotSent проверяет, что поле temperature не отправляется
// по Anthropic-протоколу, даже если задано в запросе.
func TestAnthropicTemperatureNotSent(t *testing.T) {
	var captured map[string]any
	srv := captureAnthropic(t, &captured)
	defer srv.Close()

	rm := ResolvedModel{
		Endpoint: Endpoint{Name: "ep", Kind: KindAnthropic, BaseURL: srv.URL, APIKey: "k"},
		Model:    Model{Name: "claude-test", Endpoint: "ep", MaxTokens: 100},
	}
	req := ChatRequest{
		Temperature: 0.7,
		Messages:    []Message{UserText("привет")},
	}

	_, err := completeAnthropic(context.Background(), http.DefaultClient, rm, req)
	if err != nil {
		t.Fatalf("completeAnthropic: %v", err)
	}

	if _, ok := captured["temperature"]; ok {
		t.Errorf("temperature не должен отправляться по Anthropic-протоколу, но присутствует в теле запроса")
	}
}
