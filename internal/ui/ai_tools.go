package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ivantit66/onebase/internal/llm"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/query"
)

// aiQueryRowLimit — потолок строк, отдаваемых модели одним инструментом (контроль
// размера контекста и стоимости).
const aiQueryRowLimit = 100

// aiTools формирует набор read-only инструментов для tool-use чата и исполнитель.
// Инструменты, дающие доступ к произвольным данным, выдаются только администратору
// (как и консоль запросов). Для остальных возвращается (nil, nil) — чат отвечает
// без доступа к данным.
func (s *Server) aiTools(r *http.Request) ([]llm.Tool, llm.ToolExecutor) {
	if !s.isAdmin(r) {
		return nil, nil
	}
	tools := []llm.Tool{
		{
			Name:        "описание_данных",
			Description:  "Вернуть список доступных справочников, документов и регистров с их полями. Вызови первым, чтобы понять, что можно запросить.",
			Schema:      map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name: "выполнить_запрос",
			Description: "Выполнить запрос на языке запросов OneBase (1С-подобный, только ВЫБРАТЬ) и получить строки результата. " +
				"Для остатков и оборотов используй виртуальные таблицы регистров: РегистрНакопления.Имя.Остатки(&НаДату) и .Обороты(&Нач,&Кон). " +
				"Параметры в тексте пишутся как &Имя и передаются в поле параметры.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"запрос":    map[string]any{"type": "string", "description": "текст запроса (ВЫБРАТЬ ...)"},
					"параметры": map[string]any{"type": "object", "description": "значения параметров &Имя, напр. {\"НаДату\":\"2026-06-01\"}"},
				},
				"required": []any{"запрос"},
			},
		},
	}

	exec := func(ctx context.Context, call llm.ToolCall) llm.ToolResult {
		switch call.Name {
		case "описание_данных":
			return llm.ToolResult{ID: call.ID, Content: s.aiSchemaText()}
		case "выполнить_запрос":
			return s.aiRunQuery(ctx, call)
		default:
			return llm.ToolResult{ID: call.ID, Content: "неизвестный инструмент: " + call.Name, IsError: true}
		}
	}
	return tools, exec
}

// aiSchemaText кратко описывает доступные объекты конфигурации для модели.
func (s *Server) aiSchemaText() string {
	var b strings.Builder
	var catalogs, documents []*metadata.Entity
	for _, e := range s.reg.Entities() {
		if e.Kind == metadata.KindCatalog {
			catalogs = append(catalogs, e)
		} else if e.Kind == metadata.KindDocument {
			documents = append(documents, e)
		}
	}
	writeFields := func(fs []metadata.Field) string {
		names := make([]string, 0, len(fs))
		for _, f := range fs {
			names = append(names, f.Name)
		}
		return strings.Join(names, ", ")
	}
	if len(catalogs) > 0 {
		b.WriteString("Справочники:\n")
		for _, e := range catalogs {
			fmt.Fprintf(&b, "  %s: %s\n", e.Name, writeFields(e.Fields))
		}
	}
	if len(documents) > 0 {
		b.WriteString("Документы:\n")
		for _, e := range documents {
			fmt.Fprintf(&b, "  %s: %s\n", e.Name, writeFields(e.Fields))
		}
	}
	if regs := s.reg.Registers(); len(regs) > 0 {
		b.WriteString("Регистры накопления (доступны .Остатки/.Обороты):\n")
		for _, rg := range regs {
			fmt.Fprintf(&b, "  %s: измерения [%s]; ресурсы [%s]\n",
				rg.Name, writeFields(rg.Dimensions), writeFields(rg.Resources))
		}
	}
	if irs := s.reg.InfoRegisters(); len(irs) > 0 {
		b.WriteString("Регистры сведений (доступен .СрезПоследних):\n")
		for _, ir := range irs {
			fmt.Fprintf(&b, "  %s: измерения [%s]; ресурсы [%s]\n",
				ir.Name, writeFields(ir.Dimensions), writeFields(ir.Resources))
		}
	}
	if b.Len() == 0 {
		return "В конфигурации нет объектов для запроса."
	}
	return b.String()
}

// aiRunQuery компилирует и выполняет запрос инструмента, возвращая строки в JSON.
func (s *Server) aiRunQuery(ctx context.Context, call llm.ToolCall) llm.ToolResult {
	qtext, _ := call.Input["запрос"].(string)
	qtext = stripQueryQuotes(strings.TrimSpace(qtext))
	if qtext == "" {
		return llm.ToolResult{ID: call.ID, Content: "пустой запрос", IsError: true}
	}
	var params map[string]any
	if p, ok := call.Input["параметры"].(map[string]any); ok {
		params = p
		coerceParams(params)
	}
	res, err := query.Compile(qtext, query.CompileOpts{
		Params:      params,
		Entities:    s.reg.Entities(),
		Registers:   s.reg.Registers(),
		InfoRegs:    s.reg.InfoRegisters(),
		AccountRegs: s.reg.AccountRegisters(),
		Dialect:     s.store.Dialect(),
	})
	if err != nil {
		return llm.ToolResult{ID: call.ID, Content: "ошибка компиляции запроса: " + err.Error(), IsError: true}
	}
	rows, err := s.store.QueryAll(ctx, res.SQL, res.Args...)
	if err != nil {
		return llm.ToolResult{ID: call.ID, Content: "ошибка выполнения: " + err.Error(), IsError: true}
	}
	truncated := false
	if len(rows) > aiQueryRowLimit {
		rows = rows[:aiQueryRowLimit]
		truncated = true
	}
	for _, row := range rows {
		for k, v := range row {
			if t, ok := v.(time.Time); ok {
				row[k] = t.Format("2006-01-02")
			}
		}
	}
	payload := map[string]any{"строк": len(rows), "данные": rows}
	if truncated {
		payload["усечено"] = true
		payload["примечание"] = fmt.Sprintf("показаны первые %d строк", aiQueryRowLimit)
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return llm.ToolResult{ID: call.ID, Content: "ошибка сериализации результата", IsError: true}
	}
	return llm.ToolResult{ID: call.ID, Content: string(out)}
}
