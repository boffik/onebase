// Package aiassist связывает DSL-интерфейс ИИ-помощника (interpreter.AIAssistant)
// с ядром internal/llm и конфигом базы. Вынесен отдельно, чтобы ни interpreter,
// ни llm не зависели от storage напрямую (план 48).
package aiassist

import (
	"context"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/llm"
)

// ConfigSource отдаёт текущий LLM-конфиг базы. Реализуется *storage.DB
// (метод GetLLMConfig), но интерфейс упрощает тестирование без БД.
type ConfigSource interface {
	GetLLMConfig(ctx context.Context) (llm.Config, error)
}

// Assistant реализует interpreter.AIAssistant. Конфиг перечитывается на каждый
// вызов — изменения в настройках подхватываются без перезапуска.
type Assistant struct {
	src  ConfigSource
	ctx  context.Context
	logf llm.Logf
}

// New создаёт помощника. ctx используется как родительский для запросов к
// провайдеру; logf может быть nil.
func New(ctx context.Context, src ConfigSource, logf llm.Logf) *Assistant {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Assistant{src: src, ctx: ctx, logf: logf}
}

// Configured сообщает, готов ли помощник к работе (включён и есть модели).
func (a *Assistant) Configured() bool {
	if a == nil || a.src == nil {
		return false
	}
	cfg, err := a.src.GetLLMConfig(a.ctx)
	return err == nil && cfg.Enabled && len(cfg.Models) > 0
}

// Ask выполняет запрос к ИИ согласно профилю req.Task, с фолбэком из конфига.
func (a *Assistant) Ask(req interpreter.AIRequest) (string, error) {
	cfg, err := a.src.GetLLMConfig(a.ctx)
	if err != nil {
		return "", err
	}
	runner := llm.New(cfg, a.logf)

	parts := []llm.Part{llm.TextPart(req.Prompt)}
	if req.ImageB64 != "" {
		parts = append(parts, llm.ImagePart(req.ImageB64, req.MimeType))
	}
	chatReq := llm.ChatRequest{
		System:      req.System,
		Temperature: req.Temperature,
		JSON:        req.JSON,
		Messages:    []llm.Message{{Role: llm.RoleUser, Parts: parts}},
	}
	resp, err := runner.Run(a.ctx, req.Task, chatReq)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}
