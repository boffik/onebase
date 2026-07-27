package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/storage"
)

func renderLiveList(t *testing.T, ent *metadata.Entity) string {
	t.Helper()
	data := map[string]any{
		"Entity":           ent,
		"Rows":             []map[string]any{{"id": "11111111-1111-1111-1111-111111111111", "Наименование": "X"}},
		"Params":           storage.ListParams{},
		"RefFilterOptions": map[string]any{},
		"IsAdmin":          true, "CanWrite": true, "CanDelete": true, "CanUnpost": true,
		"Lang": "ru", "Total": 1, "Page": 1, "TotalPages": 1,
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "page-list", data); err != nil {
		t.Fatalf("ExecuteTemplate page-list: %v", err)
	}
	return buf.String()
}

// TestPageList_LiveListAttributes — план 87, ступень A: контейнер списка сущности
// с list_refresh_on помечается data-ob-refresh-on (+ data-ob-live ключ), а без
// него список остаётся статичным (атрибута нет).
func TestPageList_LiveListAttributes(t *testing.T) {
	live := &metadata.Entity{
		Name: "Заказ", Kind: metadata.KindDocument, NotifyChanges: true,
		ListRefreshOn: []string{"данные.заказ", "заказ.изменён"},
		Fields:        []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	html := renderLiveList(t, live)
	if !strings.Contains(html, `data-ob-refresh-on="данные.заказ заказ.изменён"`) {
		t.Error("живой список не помечен data-ob-refresh-on с именами событий")
	}
	if !strings.Contains(html, `data-ob-live="document/заказ"`) {
		t.Error("живой список не помечен data-ob-live (ключ перечитывания)")
	}

	static := &metadata.Entity{
		Name: "Склад", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	if strings.Contains(renderLiveList(t, static), "data-ob-refresh-on") {
		t.Error("статичный список (без list_refresh_on) не должен нести data-ob-refresh-on")
	}
}
