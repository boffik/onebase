package ui

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/metadata"
)

// parseRegFilter должен собирать отбор из query: flt_<Измерение> для измерений,
// from/to (для periodic) — границы периода. Issue #45.
func TestParseRegFilter_DimsAndPeriod(t *testing.T) {
	fields := []metadata.Field{
		{Name: "Номенклатура", Type: metadata.FieldType("reference:Товары"), RefEntity: "Товары"},
		{Name: "Склад", Type: metadata.FieldTypeString},
	}
	r := httptest.NewRequest("GET", "/ui/register/остатки?flt_Номенклатура=abc-123&flt_Склад=Главный&from=2026-01-01&to=2026-06-30", nil)

	f := parseRegFilter(r, fields, true)

	if f.Dims["Номенклатура"] != "abc-123" {
		t.Errorf("Номенклатура: ожидалось abc-123, получено %q", f.Dims["Номенклатура"])
	}
	if f.Dims["Склад"] != "Главный" {
		t.Errorf("Склад: ожидалось Главный, получено %q", f.Dims["Склад"])
	}
	if f.From == nil || !f.From.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)) {
		t.Errorf("From распарсен неверно: %v", f.From)
	}
	// To должен быть концом дня (23:59:59.999999999), чтобы включать весь день.
	wantTo := time.Date(2026, 6, 30, 23, 59, 59, 999999999, time.Local)
	if f.To == nil || !f.To.Equal(wantTo) {
		t.Errorf("To распарсен неверно (ожидался конец дня): %v", f.To)
	}
}

// Пустые значения отбора не должны попадать в Dims.
func TestParseRegFilter_SkipsEmpty(t *testing.T) {
	fields := []metadata.Field{{Name: "Склад", Type: metadata.FieldTypeString}}
	r := httptest.NewRequest("GET", "/ui/register/остатки?flt_Склад=", nil)

	f := parseRegFilter(r, fields, true)
	if len(f.Dims) != 0 {
		t.Errorf("пустое значение не должно попадать в Dims: %v", f.Dims)
	}
	if !f.IsEmpty() {
		t.Errorf("фильтр без значений должен быть пустым")
	}
}

// Для непериодического регистра from/to игнорируются.
func TestParseRegFilter_NonPeriodicIgnoresPeriod(t *testing.T) {
	fields := []metadata.Field{{Name: "Склад", Type: metadata.FieldTypeString}}
	r := httptest.NewRequest("GET", "/ui/inforeg/x?from=2026-01-01&to=2026-06-30", nil)

	f := parseRegFilter(r, fields, false)
	if f.From != nil || f.To != nil {
		t.Errorf("для непериодического регистра период не должен парситься: from=%v to=%v", f.From, f.To)
	}
	if !f.IsEmpty() {
		t.Errorf("фильтр должен быть пустым")
	}
}

// TestParseRegFilter_ToIsEndOfDay проверяет, что граница «по дату» включает
// весь день: To должен быть 23:59:59.999999999 (конец дня), а не полночь.
// RED до фикса parseRegFilter.
func TestParseRegFilter_ToIsEndOfDay(t *testing.T) {
	fields := []metadata.Field{{Name: "Склад", Type: metadata.FieldTypeString}}
	r := httptest.NewRequest("GET", "/ui/register/остатки?to=2026-06-12", nil)

	f := parseRegFilter(r, fields, true)

	if f.To == nil {
		t.Fatal("To не должен быть nil")
	}
	wantEndOfDay := time.Date(2026, 6, 12, 23, 59, 59, 999999999, time.Local)
	if !f.To.Equal(wantEndOfDay) {
		t.Errorf("To должен быть концом дня (23:59:59.999999999), получено %v", f.To)
	}
	// From остаётся полночью — начало дня корректно
}

// filterFormValues возвращает текущие значения для подстановки в форму.
func TestFilterFormValues(t *testing.T) {
	fields := []metadata.Field{{Name: "Склад", Type: metadata.FieldTypeString}}
	r := httptest.NewRequest("GET", "/ui/register/остатки?flt_Склад=Главный&to=2026-06-30", nil)

	vals := filterFormValues(r, fields)
	if vals["Склад"] != "Главный" {
		t.Errorf("Склад: %q", vals["Склад"])
	}
	if vals["to"] != "2026-06-30" {
		t.Errorf("to: %q", vals["to"])
	}
	if vals["from"] != "" {
		t.Errorf("from должен быть пустым: %q", vals["from"])
	}
}
