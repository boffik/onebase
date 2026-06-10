package storage

// Тесты журнала веб-хуков _webhook_log (план 29).

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestWebhookLog_WriteAndList(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "wh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	db.LogWebhook(ctx, WebhookLogEntry{
		Webhook: "tg", Event: "document.post", Entity: "Реализация",
		RecordID: "rid-1", URL: "https://example.com/hook",
		StatusCode: 200, Duration: 120 * time.Millisecond, Attempts: 1,
	})
	db.LogWebhook(ctx, WebhookLogEntry{
		Webhook: "tg", Event: "document.post", Entity: "Реализация",
		RecordID: "rid-2", URL: "https://example.com/hook",
		StatusCode: 500, Error: "HTTP 500", Duration: 30 * time.Millisecond, Attempts: 3,
	})

	entries, err := db.ListWebhookLog(ctx, 10)
	if err != nil {
		t.Fatalf("ListWebhookLog: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ожидалось 2 записи, получено %d", len(entries))
	}
	byRecord := map[string]WebhookLogEntry{}
	for _, e := range entries {
		byRecord[e.RecordID] = e
	}
	e2 := byRecord["rid-2"]
	if e2.StatusCode != 500 || e2.Error != "HTTP 500" || e2.Attempts != 3 {
		t.Fatalf("неожиданная запись: %+v", e2)
	}
	e1 := byRecord["rid-1"]
	if e1.StatusCode != 200 || e1.DurationMs != 120 || e1.Webhook != "tg" {
		t.Fatalf("неожиданная запись: %+v", e1)
	}
	if e1.At.IsZero() {
		t.Fatal("время не заполнено")
	}
}
