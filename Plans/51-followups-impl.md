# Реализация follow-up'ов плана 51 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть три пункта плана 51 — tool-use для OpenAI/Gemini, два мелких зазора RBAC и нормальный UI настроек провайдеров/моделей вместо сырого JSON.

**Architecture:** Три независимые части. **A (tool-use)** — диспетчер `completeTools` по `Endpoint.Kind` + новые `completeOpenAITools`/`completeGeminiTools` по образцу `anthropic_tools.go`; чистый Go + httptest. **B (RBAC)** — одна строка в `sourcePermKind` (регбух→`register`), регбухи в матрице ролей, фильтрация `aiSchemaText` по правам; Go + unit-тесты. **C (UI)** — `cfgAdminAI` отдаёт форму вместо textarea, логика в новом `static/ai-settings.js` (IIFE), сохранение/проверка через существующие эндпоинты `ai/save`/`ai/test` (не меняются).

**Tech Stack:** Go 1.25, `net/http`+`httptest`, chi-роутер, ванильный JS, `html/template`/`fmt.Sprintf`-фрагменты. База ветки — `origin/main` (план 54/57 уже влиты).

**Порядок:** A → B → C (A и B — backend, полностью покрыты тестами и ниже риск; C — UI, итерируется в браузере). Части независимы и мёржатся отдельно.

**Проверка после каждой части:** `gofmt -l <изменённые>` (пусто), `go build ./...`, `go test ./<пакет>/`.

---

## Часть A. Tool-use для OpenAI и Gemini

**Контекст.** Сейчас `Runner.RunWithTools` (`internal/llm/run.go:76`) гоняет агентный цикл только для `KindAnthropic` (`run.go:87`: `if rm.Endpoint.Kind != KindAnthropic { continue }`), вызывая `completeAnthropicTools`. Типы (`Tool`/`ToolCall`/`ToolResult`/`ToolExecutor`, `MaxToolIterations=12`) — провайдеро-нейтральны (`internal/llm/tools.go`). Эталон цикла — `internal/llm/anthropic_tools.go`. Клиенты без tools (`openai.go`/`gemini.go`) дают шаблон формирования сообщений и парсинга usage.

### Task A1: `completeOpenAITools` — агентный цикл для OpenAI/compatible

**Files:**
- Create: `internal/llm/openai_tools.go`
- Test: `internal/llm/openai_tools_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
// internal/llm/openai_tools_test.go
package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// openaiToolServer: пока в истории нет сообщения role=tool — просит вызвать
// инструмент; после исполнения — отдаёт финальный текст.
func openaiToolServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(body), `"role":"tool"`) {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"Остаток: 42"}}],"usage":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"остаток","arguments":"{\"товар\":\"гвозди\"}"}}]}}],"usage":{}}`))
	}))
}

func TestRunWithToolsOpenAI(t *testing.T) {
	srv := openaiToolServer(t)
	defer srv.Close()
	cfg := Config{
		Enabled:   true,
		Endpoints: []Endpoint{{Name: "o", Kind: KindOpenAI, BaseURL: srv.URL, APIKey: "k"}},
		Models:    []Model{{Name: "m", Endpoint: "o"}},
		Profiles:  []Profile{{Task: "чат", Models: []string{"m"}}},
	}
	var gotCall ToolCall
	exec := func(ctx context.Context, call ToolCall) ToolResult { gotCall = call; return ToolResult{ID: call.ID, Content: "42"} }
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
		t.Fatalf("аргумент не распознан: %+v", gotCall.Input)
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что не компилируется/падает**

Run: `go test ./internal/llm/ -run TestRunWithToolsOpenAI -v`
Expected: FAIL — `undefined: completeOpenAITools` (после Task A3 он будет вызван; пока тест ловит отсутствие реализации/маршрутизации).

- [ ] **Step 3: Реализовать `completeOpenAITools`**

```go
// internal/llm/openai_tools.go
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// completeOpenAITools — агентный цикл tool-use по OpenAI chat/completions
// (покрывает OpenAI и openai-совместимые). Изображения в tool-путь не передаются
// (как и в anthropic-цикле). Аргументы инструмента у OpenAI приходят JSON-строкой.
func completeOpenAITools(ctx context.Context, hc *http.Client, rm ResolvedModel, req ChatRequest, tools []Tool, exec ToolExecutor) (ChatResponse, error) {
	base := rm.Endpoint.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	url := strings.TrimRight(base, "/") + "/chat/completions"
	headers := map[string]string{"Authorization": "Bearer " + rm.Endpoint.APIKey}

	toolDefs := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		schema := t.Schema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		toolDefs = append(toolDefs, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": t.Name, "description": t.Description, "parameters": schema,
			},
		})
	}

	messages := make([]map[string]any, 0, len(req.Messages)+4)
	if req.System != "" {
		messages = append(messages, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		var sb strings.Builder
		for _, p := range m.Parts {
			if !p.isImage() {
				sb.WriteString(p.Text)
			}
		}
		role := "user"
		if m.Role == RoleAssistant {
			role = "assistant"
		}
		messages = append(messages, map[string]any{"role": role, "content": sb.String()})
	}

	var totalIn, totalOut int
	for iter := 0; iter < MaxToolIterations; iter++ {
		body := map[string]any{
			"model":      rm.Model.Name,
			"max_tokens": maxTokens(rm.Model, req),
			"messages":   messages,
			"tools":      toolDefs,
		}
		data, err := postJSON(ctx, hc, "openai", url, body, headers, rm.Endpoint.Headers)
		if err != nil {
			return ChatResponse{}, err
		}
		var out struct {
			Choices []struct {
				Message struct {
					Content   string            `json:"content"`
					ToolCalls []json.RawMessage `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return ChatResponse{}, fmt.Errorf("openai: разбор ответа: %w", err)
		}
		totalIn += out.Usage.PromptTokens
		totalOut += out.Usage.CompletionTokens
		if len(out.Choices) == 0 {
			return ChatResponse{}, fmt.Errorf("openai: пустой ответ (нет choices)")
		}
		ch := out.Choices[0]
		if ch.FinishReason != "tool_calls" || len(ch.Message.ToolCalls) == 0 {
			return ChatResponse{Text: ch.Message.Content, Model: rm.Model.Name, InputTokens: totalIn, OutputTokens: totalOut}, nil
		}

		// Ход ассистента с tool_calls возвращаем модели как есть.
		messages = append(messages, map[string]any{
			"role": "assistant", "content": ch.Message.Content, "tool_calls": ch.Message.ToolCalls,
		})
		for _, raw := range ch.Message.ToolCalls {
			var tc struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			}
			if err := json.Unmarshal(raw, &tc); err != nil {
				continue
			}
			var input map[string]any
			if tc.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			}
			res := exec(ctx, ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: input})
			messages = append(messages, map[string]any{
				"role": "tool", "tool_call_id": tc.ID, "content": res.Content,
			})
		}
	}
	return ChatResponse{}, fmt.Errorf("openai: превышен лимит раундов инструментов (%d)", MaxToolIterations)
}
```

- [ ] **Step 4: Тест пока FAIL — `completeOpenAITools` не вызывается из `RunWithTools`**

Run: `go test ./internal/llm/ -run TestRunWithToolsOpenAI -v`
Expected: FAIL — `RunWithTools` ещё отбирает только anthropic-модели, openai-модель пропускается → деградация до `Run` → mock без tools отдаёт первый ответ (tool_calls) → текст пустой. Это ожидаемо; маршрутизация чинится в Task A3.

### Task A2: `completeGeminiTools` — агентный цикл для Gemini

**Files:**
- Create: `internal/llm/gemini_tools.go`
- Test: `internal/llm/gemini_tools_test.go`

- [ ] **Step 1: Написать падающий тест**

```go
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
```

- [ ] **Step 2: Запустить — FAIL** (`undefined: completeGeminiTools` / деградация).

Run: `go test ./internal/llm/ -run TestRunWithToolsGemini -v`

- [ ] **Step 3: Реализовать `completeGeminiTools`**

```go
// internal/llm/gemini_tools.go
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// completeGeminiTools — агентный цикл function-calling по Gemini generateContent.
// Конец цикла — ответ без functionCall в parts. Ответы функций уходят одним
// сообщением role=user с частями functionResponse.
func completeGeminiTools(ctx context.Context, hc *http.Client, rm ResolvedModel, req ChatRequest, tools []Tool, exec ToolExecutor) (ChatResponse, error) {
	base := rm.Endpoint.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent", strings.TrimRight(base, "/"), rm.Model.Name)
	headers := map[string]string{"x-goog-api-key": rm.Endpoint.APIKey}

	decls := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		schema := t.Schema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		decls = append(decls, map[string]any{"name": t.Name, "description": t.Description, "parameters": schema})
	}
	toolsBody := []map[string]any{{"functionDeclarations": decls}}

	contents := make([]map[string]any, 0, len(req.Messages)+4)
	for _, m := range req.Messages {
		var sb strings.Builder
		for _, p := range m.Parts {
			if !p.isImage() {
				sb.WriteString(p.Text)
			}
		}
		role := "user"
		if m.Role == RoleAssistant {
			role = "model"
		}
		contents = append(contents, map[string]any{"role": role, "parts": []map[string]any{{"text": sb.String()}}})
	}

	var totalIn, totalOut int
	for iter := 0; iter < MaxToolIterations; iter++ {
		body := map[string]any{
			"contents":         contents,
			"tools":            toolsBody,
			"generationConfig": map[string]any{"maxOutputTokens": maxTokens(rm.Model, req)},
		}
		if req.System != "" {
			body["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": req.System}}}
		}
		data, err := postJSON(ctx, hc, "gemini", endpoint, body, headers, rm.Endpoint.Headers)
		if err != nil {
			return ChatResponse{}, err
		}
		var out struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text         string `json:"text"`
						FunctionCall *struct {
							Name string         `json:"name"`
							Args map[string]any `json:"args"`
						} `json:"functionCall"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return ChatResponse{}, fmt.Errorf("gemini: разбор ответа: %w", err)
		}
		totalIn += out.UsageMetadata.PromptTokenCount
		totalOut += out.UsageMetadata.CandidatesTokenCount
		if len(out.Candidates) == 0 {
			return ChatResponse{}, fmt.Errorf("gemini: пустой ответ (нет candidates)")
		}

		var text strings.Builder
		var calls []ToolCall
		modelParts := make([]map[string]any, 0)
		for _, p := range out.Candidates[0].Content.Parts {
			if p.FunctionCall != nil {
				calls = append(calls, ToolCall{Name: p.FunctionCall.Name, Input: p.FunctionCall.Args})
				modelParts = append(modelParts, map[string]any{"functionCall": map[string]any{"name": p.FunctionCall.Name, "args": p.FunctionCall.Args}})
			} else if p.Text != "" {
				text.WriteString(p.Text)
				modelParts = append(modelParts, map[string]any{"text": p.Text})
			}
		}
		if len(calls) == 0 {
			return ChatResponse{Text: text.String(), Model: rm.Model.Name, InputTokens: totalIn, OutputTokens: totalOut}, nil
		}

		contents = append(contents, map[string]any{"role": "model", "parts": modelParts})
		respParts := make([]map[string]any, 0, len(calls))
		for _, c := range calls {
			res := exec(ctx, c)
			respParts = append(respParts, map[string]any{
				"functionResponse": map[string]any{"name": c.Name, "response": map[string]any{"result": res.Content}},
			})
		}
		contents = append(contents, map[string]any{"role": "user", "parts": respParts})
	}
	return ChatResponse{}, fmt.Errorf("gemini: превышен лимит раундов инструментов (%d)", MaxToolIterations)
}
```

- [ ] **Step 4: Тест пока FAIL** (маршрутизация чинится в A3).

### Task A3: Диспетчер `completeTools` + перевод `RunWithTools` на все протоколы

**Files:**
- Modify: `internal/llm/run.go` (функция `RunWithTools` `:76-116`; добавить `completeTools`)
- Modify: `internal/llm/tools_test.go` (`TestRunWithToolsNonAnthropicDegrades` `:86-109`)

- [ ] **Step 1: Добавить диспетчер `completeTools` в `run.go`** (рядом с `complete`, после `:144`)

```go
// completeTools диспетчеризует tool-use по типу endpoint'а. Поддерживаются все
// протоколы; неизвестный тип → ошибка (модель в RunWithTools пропускается фолбэком).
func completeTools(ctx context.Context, hc *http.Client, rm ResolvedModel, req ChatRequest, tools []Tool, exec ToolExecutor) (ChatResponse, error) {
	switch rm.Endpoint.Kind {
	case KindAnthropic:
		return completeAnthropicTools(ctx, hc, rm, req, tools, exec)
	case KindOpenAI, KindCompatible:
		return completeOpenAITools(ctx, hc, rm, req, tools, exec)
	case KindGemini:
		return completeGeminiTools(ctx, hc, rm, req, tools, exec)
	default:
		return ChatResponse{}, fmt.Errorf("endpoint %q: тип %q не поддерживает tool-use", rm.Endpoint.Name, rm.Endpoint.Kind)
	}
}
```

- [ ] **Step 2: Переписать тело цикла в `RunWithTools`** — заменить блок `:86-109`

Заменить:
```go
	for _, rm := range chain {
		if rm.Endpoint.Kind != KindAnthropic {
			continue // tool-use пока только для Anthropic-протокола
		}
		if rm.Endpoint.APIKey == "" {
			lastErr = fmt.Errorf("endpoint %q: не задан API-ключ", rm.Endpoint.Name)
			continue
		}
		tried++
		r.logf("llm: задача %q (tools) → модель %s", task, rm.Model.Name)
		resp, err := completeAnthropicTools(ctx, httpClient(rm.Endpoint), rm, req, tools, exec)
```
на:
```go
	for _, rm := range chain {
		if rm.Endpoint.APIKey == "" {
			lastErr = fmt.Errorf("endpoint %q: не задан API-ключ", rm.Endpoint.Name)
			continue
		}
		tried++
		r.logf("llm: задача %q (tools) → модель %s (%s)", task, rm.Model.Name, rm.Endpoint.Kind)
		resp, err := completeTools(ctx, httpClient(rm.Endpoint), rm, req, tools, exec)
```

- [ ] **Step 3: Обновить doc-комментарий `RunWithTools`** (`:72-75`)

Заменить:
```go
// RunWithTools выполняет запрос с доступными инструментами (tool-use). Цикл
// реализован для Anthropic-протокола (он же GLM через z.ai); модели иных типов в
// цепочке пропускаются. Если ни одна модель не поддерживает инструменты или
// tools пуст — деградирует до обычного Run (ответ без доступа к данным).
```
на:
```go
// RunWithTools выполняет запрос с доступными инструментами (tool-use). Цикл
// реализован для всех протоколов (anthropic/openai/compatible/gemini) через
// completeTools с фолбэком по цепочке профиля. Если tools пуст — деградирует до
// обычного Run; если все модели цепочки без ключа — тоже Run (ответ без данных).
```

- [ ] **Step 4: Обновить устаревший тест деградации** (`tools_test.go:86-109`)

Теперь openai-модель проходит через `completeOpenAITools`. Mock из старого теста возвращает обычный ответ без `tool_calls` → цикл сразу отдаёт текст. Переименовать и переписать смысл:

Заменить весь `TestRunWithToolsNonAnthropicDegrades` на:
```go
func TestRunWithToolsOpenAIPlainAnswer(t *testing.T) {
	// openai-модель с непустыми tools, но модель не зовёт инструмент → финальный
	// текст возвращается сразу (через completeOpenAITools, без деградации).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"ответ openai"}}],"usage":{}}`))
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
		t.Fatalf("ожидался прямой ответ openai, получено %q", resp.Text)
	}
}
```

- [ ] **Step 5: Запустить все тесты пакета — PASS**

Run: `go test ./internal/llm/ -v`
Expected: PASS — включая `TestRunWithToolsOpenAI`, `TestRunWithToolsGemini`, `TestRunWithToolsOpenAIPlainAnswer`, `TestRunWithTools` (anthropic), `TestRunWithToolsEmptyDelegates`.

- [ ] **Step 6: gofmt + сборка**

Run: `gofmt -l internal/llm/ && go build ./...`
Expected: пусто + успешная сборка.

- [ ] **Step 7: Commit**

```bash
git add internal/llm/openai_tools.go internal/llm/openai_tools_test.go internal/llm/gemini_tools.go internal/llm/gemini_tools_test.go internal/llm/run.go internal/llm/tools_test.go
git commit -m "feat(llm): tool-use для OpenAI и Gemini протоколов (follow-up плана 51)

Диспетчер completeTools по Endpoint.Kind + completeOpenAITools (tool_calls,
arguments как JSON-строка) и completeGeminiTools (functionDeclarations/
functionCall/functionResponse). RunWithTools больше не ограничен Anthropic.
Тесты на httptest-моках обоих протоколов.

Generated-with: Claude Code"
```

---

## Часть B. Два зазора RBAC

**Контекст.** План 54 уже даёт объектный RBAC: `query.Result.Sources` + `aiDeniedSource` (`internal/ui/ai_tools.go:112`) + режимы `GetAIDataScope`. Два зазора: (1) источники-регистры бухгалтерии получают `Kind==""` из `sourcePermKind` (`internal/query/query.go:54`) → для не-админа всегда «запрещено»; (2) `aiSchemaText` (`ai_tools.go:84`) отдаёт полную схему без фильтра по правам.

### Task B1: Регбух → право `register` в `sourcePermKind`

**Files:**
- Modify: `internal/query/query.go` (`sourcePermKind` `:54-66`)
- Test: `internal/query/sourcekind_test.go` (white-box, `package query`)

- [ ] **Step 1: Написать падающий white-box тест**

```go
// internal/query/sourcekind_test.go
package query

import "testing"

// TestSourcePermKind: тип источника → секция прав User.Has. Регистр бухгалтерии
// маппится на "register" (фикс зазора плана 54), а не на "" (иначе rbac запрещает
// его не-админам всегда).
func TestSourcePermKind(t *testing.T) {
	cases := map[string]string{
		"СПРАВОЧНИК":         "catalog",
		"CATALOG":            "catalog",
		"ДОКУМЕНТ":           "document",
		"DOCUMENT":           "document",
		"РЕГИСТРБУХГАЛТЕРИИ":  "register",
		"ACCOUNTINGREGISTER": "register",
		"НЕЧТО":              "",
	}
	for in, want := range cases {
		if got := sourcePermKind(in); got != want {
			t.Errorf("sourcePermKind(%q)=%q, хотим %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Запустить — FAIL**

Run: `go test ./internal/query/ -run TestSourcePermKind -v`
Expected: FAIL — `РЕГИСТРБУХГАЛТЕРИИ`/`ACCOUNTINGREGISTER` дают `""`, ожидается `"register"`.

- [ ] **Step 3: Добавить case в `sourcePermKind`**

В `internal/query/query.go` в `sourcePermKind` (`:54-66`) добавить case перед финальным `return ""`:
```go
	case isAccountRegType(typeUpper):
		return "register" // регбух проверяется как регистр (план 54, фикс зазора)
```
И обновить doc-комментарий функции (`:51-53`):
```go
// sourcePermKind переводит ключевое слово типа источника в секцию прав User.Has.
// Регистр бухгалтерии (РегистрБухгалтерии) проверяется по секции "register".
// Неизвестные типы → "" (проверяемой секции прав нет).
```
(`isAccountRegType` уже определён в `query.go:223`.)

- [ ] **Step 4: Запустить — PASS**

Run: `go test ./internal/query/ -run TestSourcePermKind -v`
Expected: PASS. Также прогнать `go test ./internal/query/` — существующий `TestCompile_Sources` зелёный (он проверяет catalog/document/register/inforeg, регбух там не фигурирует).

### Task B2: Регбухи в матрице прав редактора ролей

**Files:**
- Modify: `internal/launcher/roles_handlers.go` (`roleMatrixHTML` `:236-288`; `rolePermSections` `:29-33`)
- Test: `internal/launcher/roles_handlers_test.go` (создать или дополнить)

- [ ] **Step 1: Написать падающий тест**

```go
// internal/launcher/roles_handlers_test.go (если файла нет — создать с этим package)
package launcher

import (
	"strings"
	"testing"
)

// TestRoleMatrixIncludesAccountRegisters: регистры бухгалтерии попадают в матрицу
// прав под секцией «register» — чтобы админ мог выдать на них read (план 54, фикс).
func TestRoleMatrixIncludesAccountRegisters(t *testing.T) {
	data := &configuratorData{
		AccountRegisters: []cfgAccountRegister{{Name: "Хозрасчётный"}},
	}
	html := roleMatrixHTML(data)
	if !strings.Contains(html, "Хозрасчётный") {
		t.Fatalf("регбух не попал в матрицу прав: %s", html)
	}
}
```
> Перед реализацией убедиться, что у `cfgAccountRegister` есть поле `Name` (как у `cfgRegister`): `grep -n "type cfgAccountRegister" internal/launcher/configurator.go`. Если поле называется иначе — поправить тест и Step 2 соответственно.

- [ ] **Step 2: Запустить — FAIL**

Run: `go test ./internal/launcher/ -run TestRoleMatrixIncludesAccountRegisters -v`
Expected: FAIL — `roleMatrixHTML` не перебирает `data.AccountRegisters`.

- [ ] **Step 3: Добавить регбухи в `roleMatrixHTML`**

В `internal/launcher/roles_handlers.go` в `roleMatrixHTML` после цикла по `data.Registers` (`:244-246`) добавить:
```go
	for _, ar := range data.AccountRegisters {
		ents["register"] = append(ents["register"], ar.Name)
	}
```
И переименовать секцию в `rolePermSections` (`:32`), чтобы заголовок не вводил в заблуждение:
```go
	{"register", "Регистры (накопления и бухгалтерии)", []roleOp{{"read", "Чтение"}, {"write", "Запись"}}},
```

- [ ] **Step 4: Запустить — PASS**

Run: `go test ./internal/launcher/ -run TestRoleMatrixIncludesAccountRegisters -v`
Expected: PASS.

### Task B3: Фильтрация `aiSchemaText` по правам чтения

**Files:**
- Modify: `internal/ui/ai_tools.go` (`aiSchemaText` `:84-106`, вызов в `exec` `:73`, импорты `:3-16`)
- Test: `internal/ui/ai_tools_test.go` (обновить вызовы + добавить тест)

- [ ] **Step 1: Обновить существующие вызовы под новую сигнатуру + добавить падающий тест**

В `internal/ui/ai_tools_test.go`:
1. В `TestAISchemaText` (`:35`), `TestAISchemaText_NonDataObjects` (`:59`), `TestAISchemaText_TablePartsAndPosting` (`:162`) заменить `s.aiSchemaText()` на `s.aiSchemaText(context.Background())`.
2. Добавить новый тест:
```go
func TestAISchemaText_RBACFiltered(t *testing.T) {
	ctx := context.Background()
	pub := &metadata.Entity{Name: "Товар", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}}}
	secret := &metadata.Entity{Name: "Секрет", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Код", Type: metadata.FieldTypeString}}}
	s, _ := newSubmitTestServer(t, []*metadata.Entity{pub, secret})
	if err := s.store.SaveAIDataScope(ctx, storage.AIDataScopeRBAC); err != nil {
		t.Fatal(err)
	}
	// Не-админ с правом read только на «Товар».
	user := &auth.User{Login: "u", Roles: []*auth.Role{{
		Permissions: auth.Permission{Catalogs: map[string][]string{"Товар": {"read"}}},
	}}}
	uctx := auth.ContextWithUser(ctx, user)
	txt := s.aiSchemaText(uctx)
	if !strings.Contains(txt, "Товар") {
		t.Fatalf("разрешённый объект отсутствует: %s", txt)
	}
	if strings.Contains(txt, "Секрет") {
		t.Fatalf("запрещённый объект просочился в схему: %s", txt)
	}
}
```

- [ ] **Step 2: Запустить — FAIL** (старые вызовы не компилируются под новую сигнатуру → сначала меняем сигнатуру в Step 3; до этого `go test` пакета не собирается — это и есть «красный»).

Run: `go test ./internal/ui/ -run TestAISchemaText -v`
Expected: FAIL/не компилируется (сигнатура `aiSchemaText` ещё без ctx).

- [ ] **Step 3: Переписать `aiSchemaText(ctx)` с фильтром**

В `internal/ui/ai_tools.go` добавить импорт `"github.com/ivantit66/onebase/internal/metadata"` в блок (`:3-16`) и заменить `aiSchemaText` (`:84-106`) на:
```go
// aiSchemaText кратко описывает доступные объекты конфигурации для модели. В
// режиме rbac (план 54) у не-администратора из схемы исключаются объекты-данные
// (справочники/документы/регистры/инфо-регистры/регбухи) без права read —
// согласованно с фильтрацией источников в выполнить_запрос.
func (s *Server) aiSchemaText(ctx context.Context) string {
	filter := s.store.GetAIDataScope(ctx) == storage.AIDataScopeRBAC
	allow := func(kind, name string) bool { return !filter || s.canCtx(ctx, kind, name, "read") }

	ents := make([]*metadata.Entity, 0)
	for _, e := range s.reg.Entities() {
		if allow(string(e.Kind), e.Name) {
			ents = append(ents, e)
		}
	}
	regs := make([]*metadata.Register, 0)
	for _, rg := range s.reg.Registers() {
		if allow("register", rg.Name) {
			regs = append(regs, rg)
		}
	}
	iregs := make([]*metadata.InfoRegister, 0)
	for _, ir := range s.reg.InfoRegisters() {
		if allow("inforeg", ir.Name) {
			iregs = append(iregs, ir)
		}
	}
	aregs := make([]*metadata.AccountRegister, 0)
	for _, ar := range s.reg.AccountRegisters() {
		if allow("register", ar.Name) {
			aregs = append(aregs, ar)
		}
	}

	reports := make([]aicontext.NamedTitle, 0)
	for _, rp := range s.reg.Reports() {
		reports = append(reports, aicontext.NamedTitle{Name: rp.Name, Title: rp.Title})
	}
	procs := make([]aicontext.NamedTitle, 0)
	for _, p := range s.reg.Processors() {
		procs = append(procs, aicontext.NamedTitle{Name: p.Name, Title: p.Title})
	}
	return aicontext.SchemaText(aicontext.Input{
		Entities:         ents,
		Registers:        regs,
		InfoRegisters:    iregs,
		AccountRegisters: aregs,
		ChartsOfAccounts: s.reg.ChartsOfAccounts(),
		Enums:            s.reg.Enums(),
		Constants:        s.reg.Constants(),
		Reports:          reports,
		Processors:       procs,
		Journals:         s.reg.Journals(),
		Subsystems:       s.reg.Subsystems(),
	})
}
```
И обновить вызов в `exec` (`:73`): `Content: s.aiSchemaText(ctx)` (раньше `s.aiSchemaText()`).

> Объекты без секции прав в модели `Permission` (планы счетов, перечисления, константы, журналы, подсистемы, отчёты/обработки) не фильтруются — это возможности/метаданные, а не строки данных. Регбух фильтруется как `register` (согласовано с Task B1).

- [ ] **Step 4: Запустить — PASS**

Run: `go test ./internal/ui/ -run TestAISchemaText -v`
Expected: PASS — включая новый `TestAISchemaText_RBACFiltered` (Секрет отфильтрован) и старые (с `context.Background()` режим по умолчанию admin_only → `filter==false` → полная схема).

- [ ] **Step 5: gofmt + сборка + тесты пакетов**

Run: `gofmt -l internal/query/ internal/launcher/ internal/ui/ && go build ./... && go test ./internal/query/ ./internal/launcher/ ./internal/ui/`
Expected: пусто + успех.

- [ ] **Step 6: Commit**

```bash
git add internal/query/query.go internal/query/sourcekind_test.go internal/launcher/roles_handlers.go internal/launcher/roles_handlers_test.go internal/ui/ai_tools.go internal/ui/ai_tools_test.go
git commit -m "fix(rbac): регбух как право register + фильтр описание_данных по правам (план 54 follow-up)

sourcePermKind маппит РегистрБухгалтерии на секцию register (не «»), регбухи
добавлены в матрицу прав редактора ролей, aiSchemaText в режиме rbac исключает
объекты-данные без права read у не-администратора.

Generated-with: Claude Code"
```

---

## Часть C. UI настроек провайдеров/моделей

**Контекст.** `cfgAdminAI` (`internal/launcher/ai_handlers.go:45-126`) рендерит `<textarea>` с indent-JSON `Redacted()` + inline-скрипты `aiSave`/`aiTest`, затем `scopeSection` («Доступ ИИ-чата к данным», план 54). Эндпоинты `ai/save` (`:173`), `ai/test` (`:207`), `ai/datascope` (`:147`) — **не меняются**. Статика отдаётся `/static/*` (`server.go:17` `//go:embed static`, `:86` FileServer) — новый `static/ai-settings.js` подхватится автоматически. Визуальный эталон — макеты brainstorming (гибрид: задачи сверху + раскрывающиеся таблицы провайдеров/моделей; inline-правка; цепочка ↑↓; тоггл JSON).

> UI без unit-тестов; проверка — сборка + ручной прогон в браузере (Step C3). Данные конфига передаются в JS через `data-cfg` (JSON, html-escaped) на контейнере, чтобы не зависеть от порядка загрузки пересоздаваемых `<script>`.

### Task C1: `cfgAdminAI` отдаёт контейнер формы вместо textarea

**Files:**
- Modify: `internal/launcher/ai_handlers.go` (`cfgAdminAI` `:64-103`)

- [ ] **Step 1: Заменить сборку `page`** (`:64-103`) на контейнер + подключение скрипта

Заменить весь блок от `pretty, _ := json.MarshalIndent(...)` (`:64`) до закрывающего `` , html.EscapeString(string(pretty)), b.ID, b.ID)`` (`:103`) на:
```go
	initCfg, _ := json.Marshal(cfg.Redacted())

	page := fmt.Sprintf(`<div style="padding:16px">
  <h3 style="margin:0 0 6px;font-size:15px">ИИ-помощник</h3>
  <div style="font-size:11px;color:#666;margin-bottom:10px">Провайдеры, модели и маршрутизация по задачам. Распознавание документов идёт на vision-моделях (Gemini) с фолбэком; текстовые задачи — на GLM через z.ai. Ключи хранятся в служебной таблице базы и не попадают в экспорт конфигурации. API-ключи отображаются замаскированными (<code>****</code>); оставьте маску без изменений — ключ сохранится прежним.</div>
  <div id="ai-settings-root" data-base="%s" data-cfg="%s">Загрузка…</div>
</div>
<script src="/static/ai-settings.js"></script>`, b.ID, html.EscapeString(string(initCfg)))
```
Импорты `encoding/json`, `html`, `fmt` уже есть. `scopeSection` (`:107-122`) и финальный `w.Write([]byte(page + scopeSection))` (`:125`) — без изменений.

- [ ] **Step 2: Сборка**

Run: `go build ./...`
Expected: успех (страница временно покажет «Загрузка…», т.к. `ai-settings.js` ещё нет — это нормально до Task C2).

### Task C2: `static/ai-settings.js` — формы, inline-правка, цепочка, тоггл JSON

**Files:**
- Create: `internal/launcher/static/ai-settings.js`

- [ ] **Step 1: Создать файл со всем модулем**

```js
// ai-settings.js — UI настроек ИИ-помощника (провайдеры/модели/профили задач).
// Заменяет сырой JSON-редактор. Конфиг (llm.Config) держится в памяти, рендерится
// формами; «Сохранить» шлёт конфиг целиком на .../ai/save (ключи под масками ****
// объединяются с сохранёнными на сервере). Файл — IIFE, безопасен к повторному
// исполнению: cfgAdmin пересоздаёт <script> при каждом открытии админ-панели.
(function () {
  var root = document.getElementById('ai-settings-root');
  if (!root) return;
  var baseId = root.getAttribute('data-base');
  var cfg;
  try { cfg = JSON.parse(root.getAttribute('data-cfg') || '{}'); }
  catch (e) { root.innerHTML = '<div style="color:#c00">Повреждённый конфиг: ' + e.message + '</div>'; return; }
  cfg.endpoints = cfg.endpoints || [];
  cfg.models = cfg.models || [];
  cfg.profiles = cfg.profiles || [];

  var KINDS = ['anthropic', 'gemini', 'openai', 'compatible'];
  var jsonMode = false;
  var saveURL = '/bases/' + baseId + '/configurator/admin/ai/save';
  var testURL = '/bases/' + baseId + '/configurator/admin/ai/test';

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function el(html) { var d = document.createElement('div'); d.innerHTML = html; return d.firstElementChild; }
  function modelByName(n) { for (var i = 0; i < cfg.models.length; i++) if (cfg.models[i].name === n) return cfg.models[i]; return null; }

  // --- валидация: массив предупреждений ---
  function warnings() {
    var w = [], ep = {};
    cfg.endpoints.forEach(function (e) { ep[e.name] = true; });
    cfg.models.forEach(function (m) {
      if (m.endpoint && !ep[m.endpoint]) w.push('Модель «' + m.name + '» ссылается на несуществующего провайдера «' + m.endpoint + '»');
    });
    cfg.profiles.forEach(function (p) {
      (p.models || []).forEach(function (mn) {
        var m = modelByName(mn);
        if (!m) w.push('Задача «' + p.task + '» ссылается на несуществующую модель «' + mn + '»');
        else if (p.task === 'документы' && !m.vision) w.push('Задача «документы» содержит модель «' + mn + '» без vision');
      });
    });
    return w;
  }

  // --- рендер всей панели ---
  function render() {
    if (jsonMode) return renderJson();
    root.innerHTML = '';

    var head = el('<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px"></div>');
    var en = el('<label style="display:flex;gap:6px;align-items:center;font-weight:600;font-size:12px"><input type="checkbox"> ИИ-помощник включён</label>');
    en.querySelector('input').checked = !!cfg.enabled;
    en.querySelector('input').onchange = function () { cfg.enabled = this.checked; };
    head.appendChild(en);
    var jb = el('<span style="color:#64748b;font-size:11px;cursor:pointer">⚙ Показать JSON</span>');
    jb.onclick = function () { jsonMode = true; render(); };
    head.appendChild(jb);
    root.appendChild(head);

    var ws = warnings();
    if (ws.length) {
      var wb = el('<div style="background:#fef2f2;border:1px solid #fecaca;border-radius:4px;padding:6px 8px;margin-bottom:10px;font-size:11px;color:#b91c1c"></div>');
      wb.innerHTML = ws.map(function (x) { return '⚠ ' + esc(x); }).join('<br>');
      root.appendChild(wb);
    }

    root.appendChild(sectionTasks());
    root.appendChild(collapsible('Провайдеры и ключи (' + cfg.endpoints.length + ')', renderEndpoints(), true));
    root.appendChild(collapsible('Модели (' + cfg.models.length + ')', renderModels(), false));

    var foot = el('<div style="margin-top:14px;display:flex;justify-content:flex-end;align-items:center;gap:10px"><span id="ais-msg" style="font-size:11px"></span><button style="background:#16a34a;color:#fff;border:none;padding:6px 16px;border-radius:3px;cursor:pointer;font-size:12px">Сохранить</button></div>');
    foot.querySelector('button').onclick = save;
    root.appendChild(foot);
  }

  function collapsible(title, bodyNode, open) {
    var d = el('<div style="margin-top:10px"></div>');
    var hdr = el('<div style="padding:6px 8px;background:#f1f5f9;border-radius:4px;cursor:pointer;font-weight:600;font-size:12px">' + (open ? '▾ ' : '▸ ') + esc(title) + '</div>');
    var body = el('<div></div>');
    body.appendChild(bodyNode);
    body.style.display = open ? 'block' : 'none';
    hdr.onclick = function () {
      var vis = body.style.display === 'none';
      body.style.display = vis ? 'block' : 'none';
      hdr.textContent = (vis ? '▾ ' : '▸ ') + title;
    };
    d.appendChild(hdr); d.appendChild(body);
    return d;
  }

  // --- секция «Что делает ИИ» (профили задач) ---
  function sectionTasks() {
    var wrap = el('<div style="margin-top:6px"></div>');
    wrap.appendChild(el('<div style="font-weight:600;font-size:12px;letter-spacing:.03em;margin-bottom:4px">ЧТО ДЕЛАЕТ ИИ</div>'));
    var box = el('<div style="border:1px solid #e2e8f0;border-radius:4px"></div>');
    cfg.profiles.forEach(function (p, idx) { box.appendChild(taskRow(p, idx)); });
    var add = el('<div style="padding:6px 8px;color:#2563eb;cursor:pointer;font-size:12px">+ добавить задачу</div>');
    add.onclick = function () {
      var name = prompt('Имя задачи (например: анализ, чат, конфигуратор, документы):', '');
      if (!name) return;
      cfg.profiles.push({ task: name, models: [] }); render();
    };
    box.appendChild(add);
    wrap.appendChild(box);
    return wrap;
  }

  function taskRow(p, idx) {
    var row = el('<div style="border-bottom:1px solid #eee;padding:7px 8px;font-size:12px"></div>');
    var top = el('<div style="display:flex;align-items:center;gap:8px"></div>');
    top.appendChild(el('<span style="min-width:130px;font-weight:600">' + esc(p.task) + '</span>'));
    var chips = el('<span style="flex:1"></span>');
    (p.models || []).forEach(function (mn, i) {
      if (i) chips.appendChild(el('<span style="color:#94a3b8"> → </span>'));
      chips.appendChild(el('<span style="background:#eef2ff;border:1px solid #c7d2fe;border-radius:10px;padding:1px 8px">' + esc(mn) + '</span>'));
    });
    if (!(p.models || []).length) chips.appendChild(el('<span style="color:#94a3b8">нет моделей</span>'));
    top.appendChild(chips);
    var test = el('<span style="color:#2563eb;cursor:pointer">Проверить</span>');
    var out = el('<span id="ais-task-' + idx + '" style="font-size:11px"></span>');
    test.onclick = function () { runTest(p.task, out); };
    top.appendChild(test);
    top.appendChild(out);
    var edit = el('<span style="color:#94a3b8;cursor:pointer">✎</span>');
    var editor = el('<div style="display:none;margin-top:6px"></div>');
    edit.onclick = function () {
      var vis = editor.style.display === 'none';
      editor.style.display = vis ? 'block' : 'none';
      if (vis) { editor.innerHTML = ''; editor.appendChild(chainEditor(p, idx)); }
    };
    top.appendChild(edit);
    row.appendChild(top);
    row.appendChild(editor);
    return row;
  }

  // --- редактор цепочки: нумерованный список с ↑↓ ---
  function chainEditor(p, idx) {
    p.models = p.models || [];
    var box = el('<div style="border:1px solid #cbd5e1;border-radius:4px;padding:8px;max-width:420px;background:#f8fafc"></div>');
    function redraw() {
      box.innerHTML = '';
      p.models.forEach(function (mn, i) {
        var m = modelByName(mn);
        var line = el('<div style="display:flex;align-items:center;gap:8px;padding:3px 0"></div>');
        line.appendChild(el('<span style="color:#94a3b8;width:16px">' + (i + 1) + '.</span>'));
        var label = '<b>' + esc(mn) + '</b>' + (m && m.vision ? ' <span style="background:#dcfce7;border-radius:8px;padding:0 6px;font-size:10px">vision</span>' : '') + (i ? ' <span style="color:#94a3b8">— фолбэк</span>' : '');
        line.appendChild(el('<span style="flex:1">' + label + '</span>'));
        var up = el('<span style="cursor:pointer;color:' + (i ? '#475569' : '#cbd5e1') + '">↑</span>');
        var dn = el('<span style="cursor:pointer;color:' + (i < p.models.length - 1 ? '#475569' : '#cbd5e1') + '">↓</span>');
        var rm = el('<span style="cursor:pointer;color:#c00">✕</span>');
        up.onclick = function () { if (i) { var t = p.models[i - 1]; p.models[i - 1] = p.models[i]; p.models[i] = t; redraw(); } };
        dn.onclick = function () { if (i < p.models.length - 1) { var t = p.models[i + 1]; p.models[i + 1] = p.models[i]; p.models[i] = t; redraw(); } };
        rm.onclick = function () { p.models.splice(i, 1); redraw(); };
        line.appendChild(up); line.appendChild(dn); line.appendChild(rm);
        box.appendChild(line);
      });
      var addWrap = el('<div style="margin-top:6px;border-top:1px dashed #cbd5e1;padding-top:6px"></div>');
      var sel = el('<select style="width:100%;padding:3px 6px;border:1px solid #cbd5e1;border-radius:3px;font-size:12px"></select>');
      sel.appendChild(el('<option value="">+ добавить модель…</option>'));
      cfg.models.forEach(function (m) {
        if (p.models.indexOf(m.name) >= 0) return;
        sel.appendChild(el('<option value="' + esc(m.name) + '">' + esc(m.name) + (m.vision ? ' ✓vision' : '') + '</option>'));
      });
      sel.onchange = function () { if (this.value) { p.models.push(this.value); redraw(); } };
      addWrap.appendChild(sel);
      box.appendChild(addWrap);
      // обновляем чипы в шапке задачи
      render();
      // повторно открыть редактор после render (render пересобирает DOM)
    }
    redraw();
    return box;
  }

  // --- таблица провайдеров ---
  function renderEndpoints() {
    var t = el('<div style="border:1px solid #e2e8f0;border-top:none"></div>');
    t.appendChild(rowHTML(['Имя', 'Тип', 'Base URL', 'Ключ', ''], true));
    cfg.endpoints.forEach(function (e, i) { t.appendChild(endpointRow(e, i)); });
    var add = el('<div style="padding:6px 8px;color:#2563eb;cursor:pointer;font-size:12px">+ добавить провайдера</div>');
    add.onclick = function () { cfg.endpoints.push({ name: 'new', kind: 'anthropic', base_url: '', api_key: '' }); render(); };
    t.appendChild(add);
    return t;
  }
  function endpointRow(e, i) {
    var r = el('<div style="display:flex;padding:5px 8px;align-items:center;font-size:12px' + (i % 2 ? ';background:#f9fafb' : '') + '"></div>');
    r.appendChild(el('<span style="flex:2">' + esc(e.name) + '</span>'));
    r.appendChild(el('<span style="flex:2">' + esc(e.kind) + '</span>'));
    r.appendChild(el('<span style="flex:3">' + esc(e.base_url || '—') + '</span>'));
    r.appendChild(el('<span style="flex:2">' + esc(e.api_key || '') + '</span>'));
    var act = el('<span style="width:46px;color:#94a3b8;cursor:pointer">✎ 🗑</span>');
    act.onclick = function () { editEndpoint(r, e, i); };
    r.appendChild(act);
    return r;
  }
  function editEndpoint(r, e, i) {
    r.innerHTML = '';
    r.style.background = '#fffbeb';
    var name = inp(e.name, 2), kind = sel(KINDS, e.kind, 2), url = inp(e.base_url, 3), key = inp(e.api_key, 2);
    [name, kind, url, key].forEach(function (x) { r.appendChild(x); });
    var ok = el('<span style="color:#16a34a;cursor:pointer">✓</span>');
    var del = el('<span style="color:#c00;cursor:pointer;margin-left:6px" title="удалить">🗑</span>');
    ok.onclick = function () { e.name = name.value; e.kind = kind.value; e.base_url = url.value; e.api_key = key.value; render(); };
    del.onclick = function () { cfg.endpoints.splice(i, 1); render(); };
    r.appendChild(el('<span style="width:46px"></span>')).append(ok, del);
  }

  // --- таблица моделей ---
  function renderModels() {
    var t = el('<div style="border:1px solid #e2e8f0;border-top:none"></div>');
    t.appendChild(rowHTML(['Имя', 'Провайдер', 'Vision', 'MaxTokens', ''], true));
    cfg.models.forEach(function (m, i) { t.appendChild(modelRow(m, i)); });
    var add = el('<div style="padding:6px 8px;color:#2563eb;cursor:pointer;font-size:12px">+ добавить модель</div>');
    add.onclick = function () { cfg.models.push({ name: 'new-model', endpoint: (cfg.endpoints[0] || {}).name || '', vision: false, max_tokens: 0 }); render(); };
    t.appendChild(add);
    return t;
  }
  function modelRow(m, i) {
    var r = el('<div style="display:flex;padding:5px 8px;align-items:center;font-size:12px' + (i % 2 ? ';background:#f9fafb' : '') + '"></div>');
    r.appendChild(el('<span style="flex:3">' + esc(m.name) + '</span>'));
    r.appendChild(el('<span style="flex:2">' + esc(m.endpoint) + '</span>'));
    r.appendChild(el('<span style="flex:1">' + (m.vision ? '✓' : '—') + '</span>'));
    r.appendChild(el('<span style="flex:1">' + (m.max_tokens || '') + '</span>'));
    var act = el('<span style="width:46px;color:#94a3b8;cursor:pointer">✎ 🗑</span>');
    act.onclick = function () { editModel(r, m, i); };
    r.appendChild(act);
    return r;
  }
  function editModel(r, m, i) {
    r.innerHTML = '';
    r.style.background = '#fffbeb';
    var name = inp(m.name, 3);
    var epNames = cfg.endpoints.map(function (e) { return e.name; });
    var ep = sel(epNames, m.endpoint, 2);
    var vis = el('<span style="flex:1"><input type="checkbox"' + (m.vision ? ' checked' : '') + '></span>');
    var tok = inp(m.max_tokens || '', 1);
    [name, ep, vis, tok].forEach(function (x) { r.appendChild(x); });
    var ok = el('<span style="color:#16a34a;cursor:pointer">✓</span>');
    var del = el('<span style="color:#c00;cursor:pointer;margin-left:6px">🗑</span>');
    ok.onclick = function () {
      m.name = name.value; m.endpoint = ep.value; m.vision = vis.querySelector('input').checked;
      m.max_tokens = parseInt(tok.value, 10) || 0; render();
    };
    del.onclick = function () { cfg.models.splice(i, 1); render(); };
    var box = el('<span style="width:46px"></span>'); box.append(ok, del); r.appendChild(box);
  }

  // --- мелкие хелперы полей ---
  function inp(v, flex) { var i = el('<input style="flex:' + flex + ';min-width:0;padding:3px 6px;border:1px solid #cbd5e1;border-radius:3px;font-size:12px">'); i.value = v == null ? '' : v; return i; }
  function sel(opts, cur, flex) {
    var s = el('<select style="flex:' + flex + ';padding:3px 6px;border:1px solid #cbd5e1;border-radius:3px;font-size:12px"></select>');
    opts.forEach(function (o) { s.appendChild(el('<option' + (o === cur ? ' selected' : '') + '>' + esc(o) + '</option>')); });
    return s;
  }
  function rowHTML(cols, head) {
    var r = el('<div style="display:flex;padding:4px 8px;font-size:12px' + (head ? ';background:#f8fafc;font-weight:600' : '') + '"></div>');
    var flexes = [2, 2, 3, 2, 1];
    cols.forEach(function (c, i) { r.appendChild(el('<span style="flex:' + (flexes[i] || 1) + '">' + esc(c) + '</span>')); });
    return r;
  }

  // --- режим сырого JSON ---
  function renderJson() {
    root.innerHTML = '';
    var bar = el('<div style="display:flex;justify-content:space-between;margin-bottom:8px"><span style="font-size:11px;color:#666">Режим JSON — правьте конфиг целиком</span><span style="color:#2563eb;cursor:pointer;font-size:11px">▦ Вернуть формы</span></div>');
    bar.querySelector('span:last-child').onclick = function () {
      try { var v = JSON.parse(ta.value); cfg = v; cfg.endpoints = cfg.endpoints || []; cfg.models = cfg.models || []; cfg.profiles = cfg.profiles || []; jsonMode = false; render(); }
      catch (e) { msg('Некорректный JSON: ' + e.message, '#c00'); }
    };
    var ta = el('<textarea spellcheck="false" style="width:100%;height:340px;font-family:monospace;font-size:12px;padding:8px;border:1px solid #cbd5e1;border-radius:4px;resize:vertical"></textarea>');
    ta.value = JSON.stringify(cfg, null, 2);
    var foot = el('<div style="margin-top:10px;display:flex;justify-content:flex-end;gap:10px"><span id="ais-msg" style="font-size:11px"></span><button style="background:#16a34a;color:#fff;border:none;padding:6px 16px;border-radius:3px;cursor:pointer;font-size:12px">Сохранить</button></div>');
    foot.querySelector('button').onclick = function () {
      try { cfg = JSON.parse(ta.value); save(); } catch (e) { msg('Некорректный JSON: ' + e.message, '#c00'); }
    };
    root.appendChild(bar); root.appendChild(ta); root.appendChild(foot);
  }

  function msg(text, color) { var m = document.getElementById('ais-msg'); if (m) { m.textContent = text; m.style.color = color; } }

  // --- сохранение и проверка через существующие эндпоинты ---
  function save() {
    msg('Сохранение…', '#666');
    fetch(saveURL, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cfg) })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        if (d.ok) { msg('Сохранено', '#16a34a'); if (typeof window.cfgAiRefresh === 'function') window.cfgAiRefresh(); }
        else msg(d.error || 'Ошибка', '#c00');
      })
      .catch(function () { msg('Ошибка сети', '#c00'); });
  }
  function runTest(task, out) {
    out.textContent = ' запрос…'; out.style.color = '#666';
    fetch(testURL, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ config: cfg, task: task }) })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        if (d.ok) { out.textContent = ' ✓ ответила ' + d.model; out.style.color = '#16a34a'; }
        else { out.textContent = ' ✕ ' + (d.error || 'ошибка'); out.style.color = '#c00'; }
      })
      .catch(function () { out.textContent = ' ✕ ошибка сети'; out.style.color = '#c00'; });
  }

  render();
})();
```

> Примечание для исполнителя: `taskRow` держит ссылку на `out` (место под результат «Проверить») внутри `top`; при правке цепочки `chainEditor.redraw()` зовёт общий `render()` — после него редактор закрывается (DOM пересобран), это допустимо для первой версии. Если в браузере поведение «прыгает» — вынести обновление чипов задачи в локальную перерисовку строки вместо полного `render()`. Точная вёрстка сверяется с макетами brainstorming.

- [ ] **Step 2: Сборка (статика встроится через `//go:embed static`)**

Run: `go build ./...`
Expected: успех. Файл попадёт в бинарь автоматически.

### Task C3: Ручная проверка в браузере

- [ ] **Step 1: Поднять конфигуратор и открыть страницу**

Запустить лаунчер/конфигуратор (как в проекте — `onebase` лаунчер), открыть базу → меню → «ИИ-помощник». На пустой базе показывается заготовка (`starterLLMConfig`) формами.

- [ ] **Step 2: Прогнать сценарии**
  - добавить/править/удалить провайдера (inline ✎ → ✓/✕), модель (vision-чекбокс, MaxTokens);
  - у задачи «документы» открыть ✎ → цепочка `gemini-2.5-flash → gemini-2.0-flash`, ↑↓ меняют порядок, ✕ убирает, «+ добавить модель» добавляет;
  - предупреждение появляется при ссылке на несуществующую модель/провайдера и при не-vision модели в «документы»;
  - «⚙ Показать JSON» ↔ «▦ Вернуть формы» не теряет данные; невалидный JSON даёт сообщение;
  - «Сохранить» → «Сохранено», перезайти на страницу — данные на месте, ключи под `****`;
  - «Проверить» у задачи возвращает «✓ ответила <модель>» (при настроенном ключе) или понятную ошибку;
  - секция «Доступ ИИ-чата к данным» (admin_only/rbac/all) на месте и сохраняется.

- [ ] **Step 3: Commit**

```bash
git add internal/launcher/ai_handlers.go internal/launcher/static/ai-settings.js
git commit -m "feat(configurator): нормальный UI настроек ИИ — формы провайдеров/моделей/задач вместо JSON (план 51)

cfgAdminAI отдаёт контейнер + static/ai-settings.js (IIFE): таблицы Endpoints/
Models, секция задач с цепочкой моделей и Проверить, inline-правка строк,
редактор цепочки ↑↓, тоггл сырого JSON, клиентская валидация ссылок. Сохранение
и проверка — через существующие ai/save и ai/test (UnmaskKeys на сервере).

Generated-with: Claude Code"
```

---

## Финальная проверка (после всех частей)

- [ ] `gofmt -l internal/llm/ internal/query/ internal/launcher/ internal/ui/` — пусто
- [ ] `go build ./...` — успех
- [ ] `go test ./...` — зелёный (новые тесты `internal/llm`, `internal/query`, `internal/launcher`, `internal/ui` + существующие)
- [ ] `git log --oneline origin/main..HEAD` — спека + три коммита частей

---

## Self-Review (выполнено автором плана)

**1. Покрытие спеки:**
- UI (форма, inline, цепочка ↑↓, тоггл JSON, валидация, scopeSection сохранён, save/test без изменений) → Часть C (C1–C3). ✓
- Tool-use OpenAI/Gemini (диспетчер + два клиента + тесты, деградация) → Часть A (A1–A3). ✓
- RBAC: регбух→register (sourcePermKind + матрица ролей), фильтр описание_данных → Часть B (B1–B3). ✓
- «Не трогаем»: хранилище/Redacted/UnmaskKeys/формат Config/vision-в-tools — соблюдено (tool-путь текстовый; save/test/datascope эндпоинты не меняются). ✓

**2. Плейсхолдеры:** код приведён целиком для backend (A, B) и для `ai-settings.js`/handler (C). UI-вёрстка сверяется с макетами brainstorming; unit-тестов у JS нет — заменено ручным чек-листом C3 (UI без модульных тестов — осознанно).

**3. Согласованность типов/имён:** `completeTools`/`completeOpenAITools`/`completeGeminiTools` (одинаковая сигнатура с `completeAnthropicTools`); `ToolCall.Input map[string]any` (OpenAI — `json.Unmarshal` строки arguments; Gemini — `args` напрямую); `aiSchemaText(ctx)` — сигнатура и все три старых вызова обновлены; `sourcePermKind` использует существующий `isAccountRegType`; `auth.Permission`/`auth.Role`/`auth.User`/`auth.ContextWithUser` — по существующему API.

**4. Неоднозначности:** место хранения JS (решено: `/static/ai-settings.js` через существующий `//go:embed static`); передача конфига в JS (через `data-cfg`, чтобы не зависеть от порядка пересоздаваемых `<script>`); проверка «Проверить» — по задаче (резолв-цепочка), переиспользует `ai/test`.
