package storage_test

import (
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/storage"
)

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		prefix string
		length int
		number int
		want   string
	}{
		{"ПОС-", 8, 1, "ПОС-00000001"},
		{"ПОС-", 8, 42, "ПОС-00000042"},
		{"РТ-", 6, 999, "РТ-000999"},
		{"", 5, 1, "00001"},
		{"", 3, 1000, "1000"}, // число длиннее length — не обрезаем
	}
	for _, c := range cases {
		got := storage.FormatNumber(c.prefix, c.length, c.number)
		if got != c.want {
			t.Errorf("FormatNumber(%q, %d, %d) = %q, want %q", c.prefix, c.length, c.number, got, c.want)
		}
	}
}

func TestComputePeriodKey_Year(t *testing.T) {
	num := &metadata.Numerator{Period: "year"}
	fields := map[string]any{
		"Дата": time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	}
	got := storage.ComputePeriodKey(num, fields)
	if got != "2026" {
		t.Errorf("expected '2026', got %q", got)
	}
}

func TestComputePeriodKey_Month(t *testing.T) {
	num := &metadata.Numerator{Period: "month"}
	fields := map[string]any{
		"Дата": time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	}
	got := storage.ComputePeriodKey(num, fields)
	if got != "2026-05" {
		t.Errorf("expected '2026-05', got %q", got)
	}
}

func TestComputePeriodKey_None(t *testing.T) {
	num := &metadata.Numerator{Period: "none"}
	fields := map[string]any{
		"Дата": time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	}
	got := storage.ComputePeriodKey(num, fields)
	if got != "" {
		t.Errorf("expected '', got %q", got)
	}
}

func TestComputePeriodKey_NoDateField(t *testing.T) {
	num := &metadata.Numerator{Period: "year"}
	fields := map[string]any{"Название": "тест"}
	// Нет поля date — берётся текущий год, не паникует
	got := storage.ComputePeriodKey(num, fields)
	if len(got) != 4 {
		t.Errorf("expected 4-digit year, got %q", got)
	}
}

// scope: Организация — отдельный счётчик у каждой организации.
func TestComputePeriodKey_ScopeWithYear(t *testing.T) {
	num := &metadata.Numerator{Period: "year", Scope: "Организация"}
	fields := map[string]any{
		"Дата":        time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		"Организация": "uuid-org-A",
	}
	got := storage.ComputePeriodKey(num, fields)
	if got != "2026|uuid-org-A" {
		t.Errorf("expected '2026|uuid-org-A', got %q", got)
	}
}

// Разные организации дают разные ключи → отдельные счётчики.
func TestComputePeriodKey_ScopeDistinguishesOrgs(t *testing.T) {
	num := &metadata.Numerator{Period: "year", Scope: "Организация"}
	keyA := storage.ComputePeriodKey(num, map[string]any{
		"Дата":        time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		"Организация": "A",
	})
	keyB := storage.ComputePeriodKey(num, map[string]any{
		"Дата":        time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		"Организация": "B",
	})
	if keyA == keyB {
		t.Errorf("разные организации дали одинаковый ключ: %q", keyA)
	}
}

// Без периода, только scope.
func TestComputePeriodKey_ScopeOnly(t *testing.T) {
	num := &metadata.Numerator{Period: "none", Scope: "Касса"}
	fields := map[string]any{"Касса": "касса-1"}
	got := storage.ComputePeriodKey(num, fields)
	if got != "касса-1" {
		t.Errorf("expected 'касса-1', got %q", got)
	}
}

// Scope с отсутствующим полем — пустая часть, но не паникует.
func TestComputePeriodKey_ScopeMissingField(t *testing.T) {
	num := &metadata.Numerator{Period: "year", Scope: "Организация"}
	fields := map[string]any{
		"Дата": time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	}
	got := storage.ComputePeriodKey(num, fields)
	if got != "2026|" {
		t.Errorf("expected '2026|', got %q", got)
	}
}
