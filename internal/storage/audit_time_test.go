package storage

import (
	"testing"
	"time"
)

// parseAuditTime должен нормализовать значение колонки at независимо от
// диалекта: PostgreSQL отдаёт time.Time, SQLite — строку/[]byte.
func TestParseAuditTime_TimeTime(t *testing.T) {
	want := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	if got := parseAuditTime(want); !got.Equal(want) {
		t.Errorf("time.Time: got %v, want %v", got, want)
	}
}

func TestParseAuditTime_SQLiteString(t *testing.T) {
	// формат datetime('now') в SQLite
	got := parseAuditTime("2026-05-21 12:30:45")
	if got.IsZero() {
		t.Fatal("SQLite-строка не распарсилась")
	}
	if got.Year() != 2026 || got.Month() != 5 || got.Day() != 21 ||
		got.Hour() != 12 || got.Minute() != 30 || got.Second() != 45 {
		t.Errorf("неверный разбор: %v", got)
	}
}

func TestParseAuditTime_RFC3339(t *testing.T) {
	got := parseAuditTime("2026-05-21T12:30:45Z")
	if got.IsZero() {
		t.Fatal("RFC3339 не распарсился")
	}
}

func TestParseAuditTime_Bytes(t *testing.T) {
	// SQLite-драйвер может вернуть TEXT как []byte
	got := parseAuditTime([]byte("2026-05-21 12:30:45"))
	if got.IsZero() {
		t.Fatal("[]byte-строка не распарсилась")
	}
}

func TestParseAuditTime_PGTextTZ(t *testing.T) {
	// текстовый формат timestamptz из pgx
	got := parseAuditTime("2026-05-21 12:30:45+00")
	if got.IsZero() {
		t.Fatal("pg text timestamptz не распарсился")
	}
}

func TestParseAuditTime_Garbage(t *testing.T) {
	if got := parseAuditTime("не дата"); !got.IsZero() {
		t.Errorf("мусор → должно быть нулевое время, got %v", got)
	}
	if got := parseAuditTime(nil); !got.IsZero() {
		t.Errorf("nil → нулевое время, got %v", got)
	}
}
