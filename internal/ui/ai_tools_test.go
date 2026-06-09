package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/llm"
	"github.com/ivantit66/onebase/internal/metadata"
)

func aiToolsTestServer(t *testing.T) *Server {
	t.Helper()
	cat := &metadata.Entity{
		Name: "Товар",
		Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Цена", Type: metadata.FieldTypeNumber},
		},
	}
	s, _ := newSubmitTestServer(t, []*metadata.Entity{cat})
	return s
}

func TestAISchemaText(t *testing.T) {
	s := aiToolsTestServer(t)
	txt := s.aiSchemaText()
	if !strings.Contains(txt, "Товар") {
		t.Fatalf("в описании нет справочника Товар: %s", txt)
	}
	if !strings.Contains(txt, "Наименование") || !strings.Contains(txt, "Цена") {
		t.Fatalf("в описании нет полей: %s", txt)
	}
}

func TestAIRunQueryValid(t *testing.T) {
	s := aiToolsTestServer(t)
	res := s.aiRunQuery(context.Background(), llm.ToolCall{
		ID:    "q1",
		Input: map[string]any{"запрос": "ВЫБРАТЬ Наименование ИЗ Справочник.Товар"},
	})
	if res.IsError {
		t.Fatalf("валидный запрос дал ошибку: %s", res.Content)
	}
	if !strings.Contains(res.Content, "строк") {
		t.Fatalf("в результате нет поля строк: %s", res.Content)
	}
}

func TestAIRunQueryInvalid(t *testing.T) {
	s := aiToolsTestServer(t)
	res := s.aiRunQuery(context.Background(), llm.ToolCall{
		ID:    "q2",
		Input: map[string]any{"запрос": "это не запрос"},
	})
	if !res.IsError {
		t.Fatalf("ожидалась ошибка на некорректный запрос, получено: %s", res.Content)
	}
}

func TestAIRunQueryEmpty(t *testing.T) {
	s := aiToolsTestServer(t)
	res := s.aiRunQuery(context.Background(), llm.ToolCall{ID: "q3", Input: map[string]any{"запрос": "   "}})
	if !res.IsError {
		t.Fatal("ожидалась ошибка на пустой запрос")
	}
}
