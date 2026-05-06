package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveTemplate_Today(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	result := resolveTemplate("{{today}}", now)
	got, ok := result.(time.Time)
	assert.True(t, ok)
	assert.Equal(t, 2026, got.Year())
	assert.Equal(t, time.May, got.Month())
	assert.Equal(t, 5, got.Day())
	assert.Equal(t, 0, got.Hour())
}

func TestResolveTemplate_MinusDays(t *testing.T) {
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	result := resolveTemplate("{{today | minus_days:7}}", now)
	got, ok := result.(time.Time)
	assert.True(t, ok)
	assert.Equal(t, time.May, got.Month())
	assert.Equal(t, 3, got.Day())
}

func TestResolveTemplate_MinusMonths(t *testing.T) {
	now := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	result := resolveTemplate("{{today | minus_months:1}}", now)
	got, ok := result.(time.Time)
	assert.True(t, ok)
	assert.Equal(t, time.April, got.Month())
}

func TestResolveTemplate_StartOfMonth(t *testing.T) {
	now := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	result := resolveTemplate("{{today | start_of_month}}", now)
	got, ok := result.(time.Time)
	assert.True(t, ok)
	assert.Equal(t, 1, got.Day())
}

func TestResolveTemplate_NoTemplate(t *testing.T) {
	now := time.Now()
	result := resolveTemplate("просто строка", now)
	assert.Equal(t, "просто строка", result)
}

func TestResolveParamTemplates_Mixed(t *testing.T) {
	now := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	params := map[string]any{
		"Дата":     "{{today | minus_days:7}}",
		"Процент":  float64(10),
		"Название": "тест",
	}
	result := resolveParamTemplatesAt(params, now)
	got, ok := result["Дата"].(time.Time)
	assert.True(t, ok)
	assert.Equal(t, 28, got.Day()) // 2026-05-05 minus 7 days = April 28
	assert.Equal(t, time.April, got.Month())
	assert.Equal(t, float64(10), result["Процент"])
	assert.Equal(t, "тест", result["Название"])
}
