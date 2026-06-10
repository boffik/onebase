package webhook

// Тесты диспетчера исходящих веб-хуков (план 29): фильтры, шаблоны тела,
// retry с экспоненциальной задержкой, журналирование.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recorder — тестовый приёмник веб-хуков.
type recorder struct {
	mu     sync.Mutex
	bodies []string
	heads  []http.Header
	fails  int32 // сколько первых запросов вернуть с 500
}

func (rec *recorder) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		rec.mu.Lock()
		rec.bodies = append(rec.bodies, sb.String())
		rec.heads = append(rec.heads, r.Header.Clone())
		rec.mu.Unlock()
		if atomic.AddInt32(&rec.fails, -1) >= 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (rec *recorder) count() int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return len(rec.bodies)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("условие не выполнилось за 5 секунд")
}

func TestDispatcher_FiresOnMatchingEvent(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	d := New([]Config{{
		Name:    "tg",
		On:      "document.post",
		Filter:  map[string]string{"entity": "Реализация"},
		URL:     srv.URL,
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json", "X-Token": "abc"},
		Body:    `{"text": "Документ {{id}} ({{entity}}) на сумму {{Сумма}} проведён пользователем {{user}}"}`,
	}}, nil)

	d.Dispatch(Event{
		Name:   "document.post",
		Entity: "Реализация",
		ID:     "11111111-2222-3333-4444-555555555555",
		User:   "ivan",
		Record: map[string]any{"Сумма": 1500},
	})
	d.Wait()

	if rec.count() != 1 {
		t.Fatalf("ожидался 1 запрос, получено %d", rec.count())
	}
	body := rec.bodies[0]
	var parsed map[string]string
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("тело не JSON: %v (%s)", err, body)
	}
	want := "Документ 11111111-2222-3333-4444-555555555555 (Реализация) на сумму 1500 проведён пользователем ivan"
	if parsed["text"] != want {
		t.Fatalf("тело: %q, ожидалось %q", parsed["text"], want)
	}
	if rec.heads[0].Get("X-Token") != "abc" {
		t.Fatal("кастомный заголовок не передан")
	}
}

func TestDispatcher_FilterAndEventMismatch(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	d := New([]Config{
		{Name: "a", On: "document.post", Filter: map[string]string{"entity": "Реализация"}, URL: srv.URL, Body: "x"},
		{Name: "b", On: "catalog.save", URL: srv.URL, Body: "y"},
	}, nil)

	// не то событие
	d.Dispatch(Event{Name: "document.save", Entity: "Реализация"})
	// то событие, но не та сущность
	d.Dispatch(Event{Name: "document.post", Entity: "Заказ"})
	d.Wait()

	if rec.count() != 0 {
		t.Fatalf("ожидалось 0 запросов, получено %d", rec.count())
	}

	// catalog.save без фильтра — срабатывает на любую сущность
	d.Dispatch(Event{Name: "catalog.save", Entity: "Контрагенты"})
	d.Wait()
	if rec.count() != 1 {
		t.Fatalf("хук без фильтра должен сработать, запросов: %d", rec.count())
	}
}

func TestDispatcher_RetriesOn5xx(t *testing.T) {
	rec := &recorder{fails: 2} // первые два запроса → 500
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	logged := make(chan LogEntry, 1)
	d := New([]Config{{
		Name: "r", On: "document.save", URL: srv.URL, Body: "x", Retry: 2,
	}}, func(e LogEntry) { logged <- e })
	d.retryBase = time.Millisecond // ускоряем экспоненту в тесте

	d.Dispatch(Event{Name: "document.save", Entity: "Заказ", ID: "id1"})
	d.Wait()

	if rec.count() != 3 {
		t.Fatalf("ожидалось 3 попытки (1 + 2 retry), получено %d", rec.count())
	}
	e := <-logged
	if e.StatusCode != 200 || e.Webhook != "r" || e.Event != "document.save" {
		t.Fatalf("лог: %+v", e)
	}
}

func TestDispatcher_LogsFailure(t *testing.T) {
	logged := make(chan LogEntry, 1)
	d := New([]Config{{
		Name: "dead", On: "document.save", URL: "http://127.0.0.1:1/unreachable", Body: "x",
	}}, func(e LogEntry) { logged <- e })
	d.retryBase = time.Millisecond

	d.Dispatch(Event{Name: "document.save", Entity: "Заказ"})
	d.Wait()

	e := <-logged
	if e.Error == "" {
		t.Fatalf("ожидалась ошибка в логе, получено %+v", e)
	}
}

// Строковые значения экранируются для безопасной вставки внутрь JSON-строк.
func TestDispatcher_EscapesJSONInStrings(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	d := New([]Config{{
		Name: "j", On: "catalog.save", URL: srv.URL,
		Body: `{"name": "{{Наименование}}"}`,
	}}, nil)

	d.Dispatch(Event{Name: "catalog.save", Entity: "Товары",
		Record: map[string]any{"Наименование": `Труба "стальная"` + "\nдвухдюймовая"}})
	d.Wait()

	if rec.count() != 1 {
		t.Fatalf("запросов: %d", rec.count())
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(rec.bodies[0]), &parsed); err != nil {
		t.Fatalf("кавычки/переводы строк сломали JSON: %v (%s)", err, rec.bodies[0])
	}
	if !strings.Contains(parsed["name"], `"стальная"`) {
		t.Fatalf("значение исказилось: %q", parsed["name"])
	}
}
