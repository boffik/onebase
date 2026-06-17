package ui

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/dsl/ast"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/page"
	"github.com/ivantit66/onebase/internal/runtime"
	"github.com/ivantit66/onebase/internal/storage"
)

// TestPageCustomTemplate отрисовывает шаблон page-custom со всеми типами блоков.
// Ловит ошибки доступа к полям и вызовов FuncMap (pageRaw/pageChart/echartsJSON)
// без поднятия полноценного сервера.
func TestPageCustomTemplate(t *testing.T) {
	b := interpreter.NewPageBuilder()
	b.CallMethod("заголовок", []any{"Заголовок"})
	b.CallMethod("абзац", []any{"Абзац"})
	b.CallMethod("показатель", []any{"KPI", 42.0, "number"})
	b.CallMethod("кнопка", []any{"Кнопка", "/ui/"})
	b.CallMethod("разделитель", nil)
	b.CallMethod("добавитьсыройhtml", []any{"<b>ok</b>"})

	tbl := b.CallMethod("таблица", []any{"Таблица"}).(*interpreter.DSLPageTable)
	tbl.CallMethod("колонки", []any{"A"})
	row := tbl.CallMethod("добавитьстроку", nil).(*interpreter.DSLPageRow)
	row.CallMethod("установить", []any{"A", "x"})
	row.CallMethod("ссылка", []any{"A", "/ui/catalog/Товар/1"})

	lst := b.CallMethod("список", []any{"Список"}).(*interpreter.DSLPageList)
	lst.CallMethod("пункт", []any{"Пункт", "/ui/"})

	ch := b.CallMethod("график", []any{"График", "line"}).(*interpreter.DSLPageChart)
	ch.CallMethod("категории", []any{"Янв", "Фев"})
	ch.CallMethod("серия", []any{"S", interpreter.NewArray([]any{1.0, 2.0})})

	var buf bytes.Buffer
	data := map[string]any{
		"PageTitle":    "Тест",
		"PageBlocks":   b.Blocks(),
		"PageHasChart": true,
		"Cfg":          Config{},
		"Lang":         "ru",
	}
	if err := tmpl.ExecuteTemplate(&buf, "page-custom", data); err != nil {
		t.Fatalf("execute page-custom: %v", err)
	}
	out := buf.String()
	// URL в href нормализуется html/template (кириллица percent-кодируется),
	// поэтому проверяем ASCII-префикс пути ячейки-ссылки.
	for _, want := range []string{"Заголовок", "<b>ok</b>", "/ui/catalog/", "data-pagechart", "echarts.min.js"} {
		if !strings.Contains(out, want) {
			t.Errorf("в выводе нет %q", want)
		}
	}
	// Сырой HTML не должен пройти экранирование (pageRaw), а текст блоков —
	// должен (нет «живого» тега из текста).
	if strings.Contains(out, "&lt;b&gt;ok") {
		t.Errorf("сырой HTML был экранирован")
	}
}

// TestPageCustomActionButton проверяет рендер кнопки-действия (план 66): она
// должна стать POST-формой на /ui/page/<имя>/action/<действие> (с сохранением
// query string), а обычная кнопка-ссылка — остаться <a href>.
func TestPageCustomActionButton(t *testing.T) {
	b := interpreter.NewPageBuilder()
	b.CallMethod("кнопка", []any{"Открыть", "/ui/catalog/Товар"})
	b.CallMethod("кнопкадействие", []any{"Пересчитать", "ПересчитатьИтоги"})

	var buf bytes.Buffer
	data := map[string]any{
		"PageTitle":      "Тест",
		"PageBlocks":     b.Blocks(),
		"PageActionBase": "/ui/page/Панель/action/",
		"PageQuery":      "?период=2026",
		"Cfg":            Config{},
		"Lang":           "ru",
	}
	if err := tmpl.ExecuteTemplate(&buf, "page-custom", data); err != nil {
		t.Fatalf("execute page-custom: %v", err)
	}
	out := buf.String()
	// Кнопка-действие — POST-форма на /action/ (ASCII-часть пути не кодируется).
	if !strings.Contains(out, `method="post"`) {
		t.Errorf("кнопка-действие должна рендериться POST-формой:\n%s", out)
	}
	if !strings.Contains(out, "/action/") {
		t.Errorf("в action-URL нет сегмента /action/:\n%s", out)
	}
	// Обычная кнопка осталась ссылкой.
	if !strings.Contains(out, `<a href="/ui/catalog/`) {
		t.Errorf("кнопка-ссылка должна остаться <a href>:\n%s", out)
	}
}

// TestPageAction_RunsProcAndRedirects проверяет обработчик кнопки-действия:
// POST /ui/page/{name}/action/{action} находит процедуру в .page.os, исполняет
// её (Сообщить копится в стор), затем PRG-редиректом 303 возвращает на страницу
// с сохранением Параметров (query string).
func TestPageAction_RunsProcAndRedirects(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	src := `Процедура ПриФормировании(Страница, Параметры) Экспорт
  Страница.Заголовок("Тест");
КонецПроцедуры

Процедура Отметить(Страница, Параметры) Экспорт
  Сообщить("действие за период: " + Параметры.Получить("period"));
КонецПроцедуры`
	prog := mustParse(t, src)

	registry := runtime.NewRegistry()
	registry.LoadPages([]*page.Page{{Name: "Тест"}})
	registry.Load(runtime.LoadOptions{
		PagePrograms: map[string]*ast.Program{"Тест": prog},
	})

	interp := interpreter.New()
	interp.LookupProc = registry.GetModuleProc

	s := &Server{
		store:    db,
		reg:      registry,
		interp:   interp,
		lockMgr:  runtime.NewLockManager(),
		messages: NewMessageStore(),
	}

	// Имя страницы/действия — percent-encoded (как в ссылках меню), чтобы заодно
	// проверить decodePathParam; query — ASCII, чтобы ассерт на Location был чистым.
	req := httptest.NewRequest("POST", "/ui/page/%D0%A2%D0%B5%D1%81%D1%82/action/%D0%9E%D1%82%D0%BC%D0%B5%D1%82%D0%B8%D1%82%D1%8C?period=2026", nil)
	// URL-параметры маршрута задаём вручную (без поднятия роутера chi).
	// Значения percent-encoded: «Тест» и «Отметить».
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "%D0%A2%D0%B5%D1%81%D1%82")
	rctx.URLParams.Add("action", "%D0%9E%D1%82%D0%BC%D0%B5%D1%82%D0%B8%D1%82%D1%8C")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	s.pageAction(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ожидался 303 See Other, получен %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "period=2026") {
		t.Errorf("редирект должен сохранять Параметры (query): %q", loc)
	}
	// Сообщить из действия — в сторе сообщений (бар поллит /ui/messages).
	msgs := s.messages.List("_anonymous")
	if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "действие за период: 2026") {
		t.Errorf("Сообщить действия не собрано: %+v", msgs)
	}
}

// TestDecodePathParam фиксирует фикс маршрута /ui/page/{name}: имя из URL должно
// декодироваться, иначе ссылка из меню (percent-encoding в нижнем регистре hex,
// при котором chi отдаёт сырой сегмент) даёт 404, хотя верхний регистр проходит.
func TestDecodePathParam(t *testing.T) {
	cases := map[string]string{
		"%d0%9f%d0%b0%d0%bd%d0%b5%d0%bb%d1%8c": "Панель", // нижний регистр — ссылки меню
		"%D0%9F%D0%B0%D0%BD%D0%B5%D0%BB%D1%8C": "Панель", // верхний регистр
		"Панель":                               "Панель", // уже декодировано (нет «%»)
		"":                                     "",       // пусто
	}
	for in, want := range cases {
		if got := decodePathParam(in); got != want {
			t.Errorf("decodePathParam(%q) = %q, хотел %q", in, got, want)
		}
	}
}
