package launcher

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/llm"
	"github.com/ivantit66/onebase/internal/storage"
)

func TestLogCfgAI_RespectsFlag(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// флаг выключен — ничего не пишем
	logCfgAI(ctx, db, llm.Config{LogHistory: false}, "admin",
		"конфигуратор-генерация", "ТЗ", "ответ", llm.ChatResponse{Model: "m"})
	if e, _ := db.ListAIAudit(ctx, 10); len(e) != 0 {
		t.Fatalf("при выключенном флаге запись не должна создаваться, есть %d", len(e))
	}

	// флаг включён — пишем запрос и ответ
	logCfgAI(ctx, db, llm.Config{LogHistory: true}, "admin",
		"конфигуратор-генерация", "ТЗ", "ответ",
		llm.ChatResponse{Model: "glm-4.6", InputTokens: 5, OutputTokens: 7})
	e, _ := db.ListAIAudit(ctx, 10)
	if len(e) != 1 || e[0].Response != "ответ" || e[0].Task != "конфигуратор-генерация" || e[0].OutputTokens != 7 {
		t.Fatalf("запись журнала неверна: %+v", e)
	}
}

func TestRenderAIHistory(t *testing.T) {
	if out := renderAIHistory(nil); !strings.Contains(out, "Журнал пуст") {
		t.Error("пустой журнал должен подсказывать про включение записи")
	}
	out := renderAIHistory([]storage.AIAuditEntry{{
		Task: "конфигуратор-генерация", Model: "glm", Query: "<b>ТЗ</b>",
		Response: "готово", InputTokens: 5, OutputTokens: 6, At: time.Now(),
	}})
	if !strings.Contains(out, "конфигуратор-генерация") || !strings.Contains(out, "готово") {
		t.Error("запись журнала не отрендерена")
	}
	if strings.Contains(out, "<b>ТЗ</b>") {
		t.Error("HTML в запросе должен экранироваться")
	}
}
