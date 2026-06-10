package ui

// Тесты лимитов и аудита ИИ-чата (план 54, этапы 2-3): rate-limit на
// пользователя, суточный потолок токенов, журнал обращений.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/llm"
	"github.com/ivantit66/onebase/internal/storage"
)

// mockAnthropicUsage — тестовый провайдер с заданным расходом токенов.
func mockAnthropicUsage(t *testing.T, reply string, in, out int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"` + reply + `"}],` +
			`"usage":{"input_tokens":` + strconv.Itoa(in) + `,"output_tokens":` + strconv.Itoa(out) + `}}`))
	}))
}

func postChat(t *testing.T, s *Server) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"messages":[{"role":"user","content":"привет"}]}`
	rr := httptest.NewRecorder()
	s.aiChat(rr, httptest.NewRequest("POST", "/ui/ai/chat", strings.NewReader(body)))
	return rr
}

func TestAIChat_RateLimited(t *testing.T) {
	s, ctx := newSubmitTestServer(t, nil)
	srv := mockAnthropicUsage(t, "ок", 1, 1)
	defer srv.Close()
	if err := s.store.SaveLLMConfig(ctx, chatConfig(srv.URL)); err != nil {
		t.Fatal(err)
	}
	s.aiChatLimit = newAIWindowLimiter(2, time.Minute)

	for i := 0; i < 2; i++ {
		if rr := postChat(t, s); rr.Code != http.StatusOK {
			t.Fatalf("запрос %d: ожидался 200, получен %d", i+1, rr.Code)
		}
	}
	rr := postChat(t, s)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("3-й запрос за минуту: ожидался 429, получен %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestAIChat_DailyTokenCap(t *testing.T) {
	s, ctx := newSubmitTestServer(t, nil)
	srv := mockAnthropicUsage(t, "ок", 1, 1)
	defer srv.Close()
	if err := s.store.SaveLLMConfig(ctx, chatConfig(srv.URL)); err != nil {
		t.Fatal(err)
	}
	if err := s.store.EnsureAIAuditSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SaveAIDailyTokenCap(ctx, 100); err != nil {
		t.Fatal(err)
	}
	// расход за сегодня уже превысил потолок
	s.store.LogAIQuery(ctx, storage.AIAuditEntry{Task: "чат", InputTokens: 90, OutputTokens: 20})

	rr := postChat(t, s)
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out.OK {
		t.Fatal("при исчерпанном суточном потолке чат должен отказывать")
	}
	if !strings.Contains(strings.ToLower(out.Error), "лимит") {
		t.Fatalf("ожидалось сообщение про лимит, получено: %q", out.Error)
	}
}

func TestAIChat_WritesAudit(t *testing.T) {
	s, ctx := newSubmitTestServer(t, nil)
	srv := mockAnthropicUsage(t, "ок", 7, 3)
	defer srv.Close()
	if err := s.store.SaveLLMConfig(ctx, chatConfig(srv.URL)); err != nil {
		t.Fatal(err)
	}
	if err := s.store.EnsureAIAuditSchema(ctx); err != nil {
		t.Fatal(err)
	}

	if rr := postChat(t, s); rr.Code != http.StatusOK {
		t.Fatalf("чат: %d (%s)", rr.Code, rr.Body.String())
	}

	entries, err := s.store.ListAIAudit(ctx, 10)
	if err != nil {
		t.Fatalf("ListAIAudit: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("после чата нет записи в журнале ИИ")
	}
	e := entries[0]
	if e.Task != "чат" || e.Model != "m" {
		t.Fatalf("неожиданная запись: %+v", e)
	}
	if e.InputTokens != 7 || e.OutputTokens != 3 {
		t.Fatalf("токены не записаны: in=%d out=%d", e.InputTokens, e.OutputTokens)
	}
}

func TestAIRunQuery_WritesAudit(t *testing.T) {
	s := aiToolsTestServer(t)
	ctx := context.Background()
	if err := s.store.EnsureAIAuditSchema(ctx); err != nil {
		t.Fatal(err)
	}

	res := s.aiRunQuery(ctx, llm.ToolCall{
		ID:    "q1",
		Input: map[string]any{"запрос": "ВЫБРАТЬ Наименование ИЗ Товар"},
	})
	if res.IsError {
		t.Fatalf("запрос упал: %s", res.Content)
	}

	entries, err := s.store.ListAIAudit(ctx, 10)
	if err != nil {
		t.Fatalf("ListAIAudit: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("aiRunQuery не записал запрос в журнал ИИ")
	}
	if !strings.Contains(entries[0].Query, "Товар") {
		t.Fatalf("в записи нет текста запроса: %+v", entries[0])
	}
}

func TestAIWindowLimiter(t *testing.T) {
	l := newAIWindowLimiter(2, 30*time.Millisecond)
	for i := 0; i < 2; i++ {
		if !l.Allow("u") {
			t.Fatalf("запрос %d должен проходить", i+1)
		}
	}
	if l.Allow("u") {
		t.Fatal("3-й запрос в окне должен блокироваться")
	}
	if !l.Allow("другой") {
		t.Fatal("лимит протёк на другой ключ")
	}
	time.Sleep(50 * time.Millisecond)
	if !l.Allow("u") {
		t.Fatal("после окна запрос должен проходить")
	}
}
