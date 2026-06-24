package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/realtime"
)

// TestEventsStream_StreamsBroadcastFrame проверяет сквозной путь SSE: подписка
// через HTTP, публикация в Hub, доставка кадра {name,data} клиенту.
func TestEventsStream_StreamsBroadcastFrame(t *testing.T) {
	hub := realtime.NewHub()
	s := &Server{hub: hub}
	srv := httptest.NewServer(http.HandlerFunc(s.eventsStream))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос /ui/events: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, ожидался text/event-stream", ct)
	}

	// Дождаться регистрации подписчика, иначе публикация уйдёт «в пустоту».
	deadline := time.Now().Add(2 * time.Second)
	for hub.SubscriberCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("подписчик не зарегистрировался за 2с")
		}
		time.Sleep(5 * time.Millisecond)
	}
	hub.Publish("*", realtime.Event{Name: "уведомление", Data: "привет"})

	frameJSON := readDataFrame(t, resp.Body)
	var frame struct {
		Name string `json:"name"`
		Data any    `json:"data"`
	}
	if err := json.Unmarshal([]byte(frameJSON), &frame); err != nil {
		t.Fatalf("кадр не разобрался как JSON: %v (%q)", err, frameJSON)
	}
	if frame.Name != "уведомление" {
		t.Fatalf("name = %q, ожидалось «уведомление»", frame.Name)
	}
	if frame.Data != "привет" {
		t.Fatalf("data = %v, ожидалось «привет»", frame.Data)
	}
}

// readDataFrame читает SSE-поток до первой строки «data:» и возвращает её
// полезную нагрузку. Комментарии («: ping») и пустые строки пропускаются.
func readDataFrame(t *testing.T, r io.Reader) string {
	t.Helper()
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("чтение SSE-потока: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "data:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}
