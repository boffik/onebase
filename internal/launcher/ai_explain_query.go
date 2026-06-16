package launcher

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/llm"
)

var aiExplainSystem = "Ты помогаешь разработчику конфигурации OneBase понять ошибки " +
	"проверки (onebase check). Объясни по-русски, кратко, что означают ошибки и как их " +
	"исправить — по пунктам, с конкретным советом. Не выдумывай: опирайся на текст ошибок."

// cfgAIExplain объясняет вывод проверки конфигурации человеческим языком.
func (h *handler) cfgAIExplain(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "Некорректный запрос"})
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeJSON(w, 400, map[string]any{"error": "Пустой запрос"})
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	cfg, err := db.GetLLMConfig(r.Context())
	if err != nil {
		writeJSON(w, 200, map[string]any{"error": "Конфиг ИИ повреждён: " + err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	runner := llm.New(cfg, nil)
	resp, err := runner.Run(ctx, "конфигуратор", llm.ChatRequest{
		System:   aiExplainSystem,
		Messages: []llm.Message{llm.UserText("Вывод проверки:\n" + req.Text)},
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{"error": llm.SafeErr(err)})
		return
	}
	logCfgAI(r.Context(), db, cfg, cfgLogin(r.Context()), "конфигуратор-объяснение", req.Text, resp.Text, resp)
	writeJSON(w, 200, map[string]any{"ok": true, "text": resp.Text, "model": resp.Model})
}

// queryHintSystem — системный промпт подсказки запроса: справочник языка + срез
// конфигурации (этап 1). Пустой schema не добавляет секцию конфигурации.
func queryHintSystem(schema string) string {
	s := "Ты строишь запрос на языке запросов OneBase (1С-подобный: ВЫБРАТЬ поля ИЗ Источник " +
		"[КАК Псевдоним] [ЛЕВОЕ СОЕДИНЕНИЕ ... ПО ...] [ГДЕ ...] [СГРУППИРОВАТЬ ПО ...] " +
		"[УПОРЯДОЧИТЬ ПО ...]). Остатки/обороты регистров — через виртуальные таблицы: " +
		"РегистрНакопления.Имя.Остатки(&НаДату), .Обороты(&Нач, &Кон); срез сведений — " +
		".СрезПоследних(&НаДату). Параметры пиши как &Имя. Используй только существующие " +
		"объекты и поля из контекста ниже. Верни ТОЛЬКО текст запроса, без пояснений и markdown."
	if strings.TrimSpace(schema) != "" {
		s += "\n\nКонфигурация базы:\n" + schema
	}
	return s
}

// cfgAIQuery строит запрос OneBase по описанию на естественном языке.
func (h *handler) cfgAIQuery(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "Некорректный запрос"})
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		writeJSON(w, 400, map[string]any{"error": "Пустой запрос"})
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	cfg, err := db.GetLLMConfig(r.Context())
	if err != nil {
		writeJSON(w, 200, map[string]any{"error": "Конфиг ИИ повреждён: " + err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	system := queryHintSystem(h.configSchemaText(ctx, b))
	runner := llm.New(cfg, nil)
	resp, err := runner.Run(ctx, "конфигуратор", llm.ChatRequest{
		System:   system,
		Messages: []llm.Message{llm.UserText(req.Description)},
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{"error": llm.SafeErr(err)})
		return
	}
	logCfgAI(r.Context(), db, cfg, cfgLogin(r.Context()), "конфигуратор-запрос", req.Description, resp.Text, resp)
	writeJSON(w, 200, map[string]any{"ok": true, "query": resp.Text, "model": resp.Model})
}
