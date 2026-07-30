package storage

import (
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/metadata"
)

// normalizeDate приводит дату-строку из SQLite к time.Time; так загруженный
// объект (Ссылка.ПолучитьОбъект) несёт настоящую Дату, как и свежесозданный.
func TestNormalizeDate(t *testing.T) {
	// RFC3339 (формат хранения даты в SQLite, см. bindFieldArg) → time.Time.
	got := normalizeDate("2026-02-20T00:00:00Z")
	tt, ok := got.(time.Time)
	if !ok {
		t.Fatalf("RFC3339-строка должна стать time.Time, получено %T", got)
	}
	if tt.Year() != 2026 || tt.Month() != time.February || tt.Day() != 20 {
		t.Fatalf("разобранная дата = %v, ожидалось 2026-02-20", tt)
	}

	// Уже time.Time (PostgreSQL) — пропускаем без изменений.
	src := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	if got := normalizeDate(src); !got.(time.Time).Equal(src) {
		t.Fatalf("time.Time должен пройти без изменений, получено %v", got)
	}

	// nil остаётся nil.
	if got := normalizeDate(nil); got != nil {
		t.Fatalf("nil должен остаться nil, получено %v", got)
	}

	// Нераспознанная строка возвращается как есть (безопасный откат).
	if got := normalizeDate("не дата"); got != "не дата" {
		t.Fatalf("нераспознанная строка должна вернуться как есть, получено %v", got)
	}
}

// normalizeFieldValue для поля-даты даёт time.Time, для числа — decimal.
func TestNormalizeFieldValue_Date(t *testing.T) {
	dateField := metadata.Field{Name: "Дата", Type: metadata.FieldTypeDate}
	got := normalizeFieldValue(dateField, "2026-01-15T00:00:00Z")
	if _, ok := got.(time.Time); !ok {
		t.Fatalf("поле-дата должно нормализоваться в time.Time, получено %T", got)
	}
}
