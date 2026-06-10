// Package webhook — исходящие веб-хуки на события платформы (план 29):
// «документ проведён → POST на URL». Конфигурируются декларативно в
// config/app.yaml (блок webhooks), отправляются асинхронно, с retry и
// журналом _webhook_log. Превращает OneBase в источник событий для
// n8n/Make/Telegram-ботов без единой строки кода.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Config — один веб-хук из app.yaml.
type Config struct {
	Name    string            `yaml:"name"`
	On      string            `yaml:"on"`     // document.save|post|unpost|delete, catalog.save|delete
	Filter  map[string]string `yaml:"filter"` // entity: ИмяСущности (пусто = все)
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"` // по умолчанию POST
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`    // шаблон: {{id}} {{entity}} {{user}} {{timestamp}} {{Поле}}
	Timeout int               `yaml:"timeout"` // секунд, по умолчанию 10
	Retry   int               `yaml:"retry"`   // повторов при ошибке, по умолчанию 0
}

// Event — событие платформы.
type Event struct {
	Name   string // document.post, catalog.save, ...
	Entity string
	ID     string
	User   string
	Record map[string]any // поля записи для шаблона тела
}

// LogEntry — запись журнала вызова веб-хука (пишется в _webhook_log).
type LogEntry struct {
	Webhook    string
	Event      string
	Entity     string
	RecordID   string
	URL        string
	StatusCode int
	Error      string
	Duration   time.Duration
	Attempts   int
}

// Dispatcher проверяет фильтры и отправляет HTTP-запросы асинхронно.
type Dispatcher struct {
	hooks     []Config
	client    *http.Client
	logFn     func(LogEntry) // best-effort журнал; может быть nil
	wg        sync.WaitGroup
	retryBase time.Duration // база экспоненциальной задержки (тесты ускоряют)
}

// New строит диспетчер. logFn вызывается после завершения каждого вызова
// (включая неудачные) — обычно это запись в _webhook_log.
func New(hooks []Config, logFn func(LogEntry)) *Dispatcher {
	return &Dispatcher{
		hooks:     hooks,
		client:    &http.Client{},
		logFn:     logFn,
		retryBase: time.Second,
	}
}

// Enabled сообщает, настроен ли хотя бы один веб-хук.
func (d *Dispatcher) Enabled() bool { return d != nil && len(d.hooks) > 0 }

// Dispatch запускает подходящие веб-хуки асинхронно: вызов не блокирует
// сохранение документа (сетевые задержки и ретраи — в фоне).
func (d *Dispatcher) Dispatch(e Event) {
	if d == nil {
		return
	}
	for i := range d.hooks {
		h := &d.hooks[i]
		if h.On != e.Name {
			continue
		}
		if want := h.Filter["entity"]; want != "" && !strings.EqualFold(want, e.Entity) {
			continue
		}
		d.wg.Add(1)
		go func(h *Config) {
			defer d.wg.Done()
			d.fire(h, e)
		}(h)
	}
}

// Wait дожидается завершения всех запущенных вызовов (graceful shutdown, тесты).
func (d *Dispatcher) Wait() { d.wg.Wait() }

// fire выполняет один веб-хук: шаблон тела → HTTP-запрос → retry → журнал.
func (d *Dispatcher) fire(h *Config, e Event) {
	start := time.Now()
	entry := LogEntry{Webhook: h.Name, Event: e.Name, Entity: e.Entity, RecordID: e.ID, URL: h.URL}

	body, err := renderBody(h.Body, e)
	if err != nil {
		entry.Error = "шаблон тела: " + err.Error()
		d.log(entry, start)
		return
	}

	method := h.Method
	if method == "" {
		method = http.MethodPost
	}
	timeout := time.Duration(h.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	attempts := h.Retry + 1
	for try := 0; try < attempts; try++ {
		entry.Attempts = try + 1
		if try > 0 {
			// экспоненциальная задержка: base, 2*base, 4*base, …
			time.Sleep(d.retryBase << (try - 1))
		}
		code, err := d.send(method, h, body, timeout)
		entry.StatusCode = code
		if err != nil {
			entry.Error = err.Error()
			continue
		}
		if code >= 200 && code < 300 {
			entry.Error = ""
			break
		}
		entry.Error = fmt.Sprintf("HTTP %d", code)
	}
	d.log(entry, start)
}

func (d *Dispatcher) send(method string, h *Config, body string, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, h.URL, strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10)) // дренируем для keep-alive
	return resp.StatusCode, nil
}

func (d *Dispatcher) log(entry LogEntry, start time.Time) {
	entry.Duration = time.Since(start)
	if d.logFn != nil {
		d.logFn(entry)
	}
}

// identRe — подстановки вида {{Поле}} (без точки, как в app.yaml плана 29).
// Преобразуются в {{index . "Поле"}} перед text/template.
var identRe = regexp.MustCompile(`\{\{\s*([\p{L}_][\p{L}\p{N}_]*)\s*\}\}`)

// renderBody подставляет переменные события в шаблон тела. Строковые значения
// экранируются по правилам JSON-строки (без обрамляющих кавычек) — кавычки и
// переводы строк в данных не ломают JSON-тело; числа/даты подставляются как есть.
func renderBody(tpl string, e Event) (string, error) {
	if tpl == "" {
		return "", nil
	}
	data := map[string]any{
		"id":        e.ID,
		"entity":    e.Entity,
		"user":      e.User,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	for k, v := range e.Record {
		if s, ok := v.(string); ok {
			data[k] = jsonEscape(s)
		} else {
			data[k] = v
		}
	}
	t, err := template.New("body").Option("missingkey=zero").Parse(
		identRe.ReplaceAllString(tpl, `{{index . "$1"}}`))
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", err
	}
	// missingkey=zero для отсутствующих ключей даёт "<no value>" — заменяем на пусто
	return strings.ReplaceAll(sb.String(), "<no value>", ""), nil
}

// jsonEscape экранирует строку для вставки внутрь JSON-строки (без кавычек).
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}
