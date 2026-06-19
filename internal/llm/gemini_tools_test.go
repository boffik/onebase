// internal/llm/gemini_tools_test.go
package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// geminiToolServer: пока в истории нет functionResponse — просит вызвать функцию;
// после — отдаёт финальный текст.
func geminiToolServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(body), "functionResponse") {
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"Остаток: 42"}]}}],"usageMetadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"остаток","args":{"товар":"гвозди"}}}]}}],"usageMetadata":{}}`))
	}))
}

func TestRunWithToolsGemini(t *testing.T) {
	srv := geminiToolServer(t)
	defer srv.Close()
	cfg := Config{
		Enabled:   true,
		Endpoints: []Endpoint{{Name: "g", Kind: KindGemini, BaseURL: srv.URL, APIKey: "k"}},
		Models:    []Model{{Name: "m", Endpoint: "g"}},
		Profiles:  []Profile{{Task: "чат", Models: []string{"m"}}},
	}
	var gotCall ToolCall
	exec := func(ctx context.Context, call ToolCall) ToolResult { gotCall = call; return ToolResult{Content: "42"} }
	tools := []Tool{{Name: "остаток", Description: "остаток товара", Schema: map[string]any{"type": "object"}}}
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
		t.Fatalf("функция не вызвана: %+v", gotCall)
	}
	if v, _ := gotCall.Input["товар"].(string); v != "гвозди" {
		t.Fatalf("аргумент не распознан: %+v", gotCall.Input)
	}
}
