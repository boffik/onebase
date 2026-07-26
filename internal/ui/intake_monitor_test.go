package ui

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
)

// seedQuarantine кладёт в БД одно карантинное событие (строка идемпотентности
// quarantined + запись DLQ с валидным конвертом для повтора).
func seedQuarantine(t *testing.T, s *Server, key string) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.store.InsertIntakeLogIfNew(ctx, storage.IntakeLogRow{
		Intake: "SiteLead", Scope: "site", Key: key, PayloadHash: "h",
		Status: storage.IntakeStatusQuarantined, ReceivedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.InsertIntakeDLQ(ctx, storage.IntakeDLQEntry{
		Intake: "SiteLead", Key: key, Scope: "site",
		Payload:       `{"event_id":"` + key + `","source":"site","payload":{"x":1}}`,
		Reason:        "handler_error", Error: "boom", QuarantinedAt: 1000,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestIntakeMonitor_Lists(t *testing.T) {
	s := newIntakeTestServer(t)
	seedQuarantine(t, s, "Q1")

	w := httptest.NewRecorder()
	s.intakeMonitor(w, httptest.NewRequest("GET", "/ui/admin/intake", nil))
	if w.Code != 200 {
		t.Fatalf("status=%d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"SiteLead", "/hs/site/lead", "Q1", "handler_error", "Повторить"} {
		if !strings.Contains(body, want) {
			t.Errorf("монитор не содержит %q", want)
		}
	}
}

func TestIntakeMonitor_Replay(t *testing.T) {
	s := newIntakeTestServer(t)
	seedQuarantine(t, s, "Q1")
	ctx := context.Background()

	form := url.Values{"intake": {"SiteLead"}, "key": {"Q1"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/ui/admin/intake/replay", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.intakeMonitorReplay(w, r)
	if w.Code != 302 {
		t.Fatalf("replay: status=%d, ожидался редирект 302; body=%s", w.Code, w.Body.String())
	}

	// Запись карантина закрыта, строка идемпотентности стала processed.
	if open, _ := s.store.ListIntakeDLQ(ctx, "SiteLead", true, 0); len(open) != 0 {
		t.Fatalf("после replay открытых записей карантина: %d, ожидалось 0", len(open))
	}
	row, _, _ := s.store.GetIntakeLog(ctx, "SiteLead", "site", "Q1")
	if row.Status != storage.IntakeStatusProcessed {
		t.Fatalf("после replay статус=%q, ожидался processed", row.Status)
	}
	if row.ResultRef != "ref-Q1" {
		t.Fatalf("после replay result_ref=%q, ожидался ref-Q1", row.ResultRef)
	}
}
