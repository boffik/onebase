package llm

import "context"

// Tool — описание инструмента для модели (function calling). Schema — JSON Schema
// объекта входных параметров.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
}

// ToolCall — намерение модели вызвать инструмент с аргументами.
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResult — результат выполнения инструмента, возвращаемый модели.
type ToolResult struct {
	ID      string
	Content string
	IsError bool
}

// ToolExecutor исполняет инструмент по запросу модели. Реализация (в слое UI)
// должна быть read-only и уважать права текущего пользователя.
type ToolExecutor func(ctx context.Context, call ToolCall) ToolResult

// MaxToolIterations — предел числа раундов tool-use в одном запросе (защита от
// зацикливания). После него возвращается то, что есть, либо ошибка.
const MaxToolIterations = 6
