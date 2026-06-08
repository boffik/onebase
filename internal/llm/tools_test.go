package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// toolServer эмулирует Anthropic с tool-use: на первом запросе (без tool_result в
// истории) просит вызвать инструмент, на втором — отдаёт финальный текст.
func toolServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		// Если в истории уже есть tool_result — значит инструмент исполнен, отвечаем текстом.
		if strings.Contains(string(body), "tool_result") {
			_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"Остаток: 42"}],"usage":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"stop_reason":"tool_use","content":[{"type":"tool_use","id":"t1","name":"остаток","input":{"товар":"гвозди"}}],"usage":{}}`))
	}))
}

func TestRunWithTools(t *testing.T) {
	srv := toolServer(t)
	defer srv.Close()

	cfg := Config{
		Enabled:   true,
		Endpoints: []Endpoint{{Name: "ep", Kind: KindAnthropic, BaseURL: srv.URL, APIKey: "k"}},
		Models:    []Model{{Name: "m", Endpoint: "ep"}},
		Profiles:  []Profile{{Task: "чат", Models: []string{"m"}}},
	}

	var gotCall ToolCall
	exec := func(ctx context.Context, call ToolCall) ToolResult {
		gotCall = call
		return ToolResult{ID: call.ID, Content: "42"}
	}
	tools := []Tool{{Name: "остаток", Description: "остаток товара", Schema: map[string]any{
		"type": "object", "properties": map[string]any{"товар": map[string]any{"type": "string"}},
	}}}

	r := New(cfg, nil)
	resp, err := r.RunWithTools(context.Background(), "чат",
		ChatRequest{Messages: []Message{UserText("сколько гвоздей?")}}, tools, exec)
	if err != nil {
		t.Fatalf("RunWithTools: %v", err)
	}
	if resp.Text != "Остаток: 42" {
		t.Fatalf("неожиданный текст: %q", resp.Text)
	}
	if gotCall.Name != "остаток" {
		t.Fatalf("инструмент не вызван корректно: %+v", gotCall)
	}
	if v, _ := gotCall.Input["товар"].(string); v != "гвозди" {
		t.Fatalf("аргумент инструмента не распознан: %+v", gotCall.Input)
	}
}

func TestRunWithToolsEmptyDelegates(t *testing.T) {
	// Пустой список инструментов → обычный Run (без tools).
	srv := anthropicServer(t, 200, "просто ответ")
	defer srv.Close()
	cfg := Config{
		Enabled:   true,
		Endpoints: []Endpoint{{Name: "ep", Kind: KindAnthropic, BaseURL: srv.URL, APIKey: "k"}},
		Models:    []Model{{Name: "m", Endpoint: "ep"}},
		Profiles:  []Profile{{Task: "чат", Models: []string{"m"}}},
	}
	r := New(cfg, nil)
	resp, err := r.RunWithTools(context.Background(), "чат",
		ChatRequest{Messages: []Message{UserText("привет")}}, nil, nil)
	if err != nil {
		t.Fatalf("RunWithTools: %v", err)
	}
	if resp.Text != "просто ответ" {
		t.Fatalf("ожидался обычный ответ, получено %q", resp.Text)
	}
}

func TestRunWithToolsNonAnthropicDegrades(t *testing.T) {
	// Профиль с openai-моделью + непустые tools → деградация до Run без инструментов.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ответ openai"}}],"usage":{}}`))
	}))
	defer srv.Close()
	cfg := Config{
		Enabled:   true,
		Endpoints: []Endpoint{{Name: "o", Kind: KindOpenAI, BaseURL: srv.URL, APIKey: "k"}},
		Models:    []Model{{Name: "m", Endpoint: "o"}},
		Profiles:  []Profile{{Task: "чат", Models: []string{"m"}}},
	}
	tools := []Tool{{Name: "x"}}
	r := New(cfg, nil)
	resp, err := r.RunWithTools(context.Background(), "чат",
		ChatRequest{Messages: []Message{UserText("привет")}}, tools, func(context.Context, ToolCall) ToolResult { return ToolResult{} })
	if err != nil {
		t.Fatalf("RunWithTools: %v", err)
	}
	if resp.Text != "ответ openai" {
		t.Fatalf("ожидалась деградация до Run, получено %q", resp.Text)
	}
}
