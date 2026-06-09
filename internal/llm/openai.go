package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// completeOpenAI вызывает OpenAI-совместимый chat/completions. Покрывает сам
// OpenAI, а также локальные/прокси-серверы (Ollama, LM Studio) сменой base_url.
func completeOpenAI(ctx context.Context, hc *http.Client, rm ResolvedModel, req ChatRequest) (ChatResponse, error) {
	base := rm.Endpoint.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	url := strings.TrimRight(base, "/") + "/chat/completions"

	// content у OpenAI — либо строка, либо массив частей (для мультимодальности).
	msgs := make([]map[string]any, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		role := "user"
		if m.Role == RoleAssistant {
			role = "assistant"
		}
		if len(m.Parts) == 1 && !m.Parts[0].isImage() {
			msgs = append(msgs, map[string]any{"role": role, "content": m.Parts[0].Text})
			continue
		}
		parts := make([]map[string]any, 0, len(m.Parts))
		for _, p := range m.Parts {
			if p.isImage() {
				parts = append(parts, map[string]any{
					"type":      "image_url",
					"image_url": map[string]any{"url": fmt.Sprintf("data:%s;base64,%s", p.MimeType, p.ImageB64)},
				})
			} else {
				parts = append(parts, map[string]any{"type": "text", "text": p.Text})
			}
		}
		msgs = append(msgs, map[string]any{"role": role, "content": parts})
	}

	body := map[string]any{
		"model":      rm.Model.Name,
		"messages":   msgs,
		"max_tokens": maxTokens(rm.Model, req),
	}
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}
	if req.JSON {
		body["response_format"] = map[string]any{"type": "json_object"}
	}

	headers := map[string]string{"Authorization": "Bearer " + rm.Endpoint.APIKey}
	data, err := postJSON(ctx, hc, "openai", url, body, headers, rm.Endpoint.Headers)
	if err != nil {
		return ChatResponse{}, err
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return ChatResponse{}, fmt.Errorf("openai: разбор ответа: %w", err)
	}
	var text string
	if len(out.Choices) > 0 {
		text = out.Choices[0].Message.Content
	}
	return ChatResponse{
		Text:         text,
		Model:        rm.Model.Name,
		InputTokens:  out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
	}, nil
}
