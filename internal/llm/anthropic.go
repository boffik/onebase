package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// anthropicSystem строит системный промпт для Anthropic Messages API. У Anthropic
// нет параметра response_format, поэтому строгий JSON задаём директивой в system
// (req.JSON). Покрывает и GLM через z.ai.
func anthropicSystem(req ChatRequest) string {
	sys := req.System
	if req.JSON {
		if sys != "" {
			sys += "\n\n"
		}
		sys += "Верни ответ строго в виде валидного JSON, без пояснений и без markdown-ограждений."
	}
	return sys
}

// completeAnthropic вызывает Anthropic Messages API. Этот же путь покрывает GLM
// через z.ai: достаточно задать endpoint base_url = https://api.z.ai/api/anthropic.
func completeAnthropic(ctx context.Context, hc *http.Client, rm ResolvedModel, req ChatRequest) (ChatResponse, error) {
	base := rm.Endpoint.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	url := strings.TrimRight(base, "/") + "/v1/messages"

	type contentBlock struct {
		Type   string         `json:"type"`
		Text   string         `json:"text,omitempty"`
		Source map[string]any `json:"source,omitempty"`
	}
	type message struct {
		Role    string         `json:"role"`
		Content []contentBlock `json:"content"`
	}

	msgs := make([]message, 0, len(req.Messages))
	for _, m := range req.Messages {
		blocks := make([]contentBlock, 0, len(m.Parts))
		for _, p := range m.Parts {
			if p.isImage() {
				blocks = append(blocks, contentBlock{Type: "image", Source: map[string]any{
					"type": "base64", "media_type": p.MimeType, "data": p.ImageB64,
				}})
			} else {
				blocks = append(blocks, contentBlock{Type: "text", Text: p.Text})
			}
		}
		msgs = append(msgs, message{Role: string(m.Role), Content: blocks})
	}

	body := map[string]any{
		"model":      rm.Model.Name,
		"max_tokens": maxTokens(rm.Model, req),
		"messages":   msgs,
	}
	if sys := anthropicSystem(req); sys != "" {
		body["system"] = sys
	}
	// temperature/top_p/top_k не отправляем по Anthropic-протоколу: Claude Opus
	// 4.7/4.8 отклоняют их (HTTP 400). Поведение модели задаём промптом.

	headers := map[string]string{
		"x-api-key":         rm.Endpoint.APIKey,
		"anthropic-version": "2023-06-01",
	}
	data, err := postJSON(ctx, hc, "anthropic", url, body, headers, rm.Endpoint.Headers)
	if err != nil {
		return ChatResponse{}, err
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: разбор ответа: %w", err)
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return ChatResponse{
		Text:         sb.String(),
		Model:        rm.Model.Name,
		InputTokens:  out.Usage.InputTokens,
		OutputTokens: out.Usage.OutputTokens,
	}, nil
}
