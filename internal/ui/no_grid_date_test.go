package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/metadata"
)

// Дата в простой таблице (no_grid) рисуется полем даты, а не текстом: в
// текстовом поле стояла сырая метка «2026-10-19T09:15:30» (история заявки
// cc2). Путь пользователя: записать строку формой, открыть запись снова —
// дата на месте, с секундами, и пересохранение формы её не меняет.
func TestNoGridDateColumn_RendersDateInputAndRoundTrips(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readOnly bool
		dateMode string
	}{
		{name: "editable"},
		{name: "readonly", readOnly: true},
		{name: "date-only", dateMode: metadata.FormDateModeDate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tp := metadata.TablePart{Name: "Lines", Fields: []metadata.Field{
				{Name: "Дата", Type: metadata.FieldTypeDate},
				{Name: "Действие", Type: metadata.FieldTypeString},
			}}
			table := &metadata.FormElement{
				Kind: metadata.FormElementTablePart, Name: "LinesTable",
				DataPath: "Object.Lines", NoGrid: true, ReadOnly: tc.readOnly,
			}
			if tc.dateMode != "" {
				table.Children = []*metadata.FormElement{{
					Kind: metadata.FormElementColumn, Name: "DateColumn",
					DataPath: "Object.Lines.Дата", DateMode: tc.dateMode,
				}}
			}
			form := managedObjectForm(fieldEl("NameField", "Object.Name"), table)
			ent := &metadata.Entity{
				Name: "NoGridDateRecord", Kind: metadata.KindCatalog,
				Fields:     []metadata.Field{{Name: "Name", Type: metadata.FieldTypeString}},
				TableParts: []metadata.TablePart{tp},
				Forms:      []*metadata.FormModule{form},
			}
			srv, ctx := newSubmitTestServer(t, []*metadata.Entity{ent})

			// Строку пишет форма: у readonly-таблицы браузер строки не шлёт,
			// поэтому её строку кладём записью с редактируемой формой.
			form.Elements[1].ReadOnly = false
			body := url.Values{
				"Name":                {"rec"},
				"tp_present.Lines":    {"1"},
				"tp.Lines.0.Дата":     {"2026-10-19T09:15:30"},
				"tp.Lines.0.Действие": {"Подтверждён отказ"},
			}
			rec := httptest.NewRecorder()
			srv.submit(rec, reqWithChi(http.MethodPost, "/ui/catalog/"+ent.Name+"/new", body,
				map[string]string{"entity": ent.Name}))
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("запись: code=%d body=%s", rec.Code, rec.Body.String())
			}
			form.Elements[1].ReadOnly = tc.readOnly
			id := onlyStoredID(t, srv, ent)

			open := func() string {
				rec := httptest.NewRecorder()
				srv.formEdit(rec, reqWithChi(http.MethodGet, "/ui/catalog/"+ent.Name+"/"+id.String(), nil,
					map[string]string{"entity": ent.Name, "id": id.String()}))
				if rec.Code != http.StatusOK {
					t.Fatalf("открытие: code=%d body=%s", rec.Code, rec.Body.String())
				}
				return rec.Body.String()
			}
			page := open()
			want := `<input type="datetime-local" step="1" name="tp.Lines.0.Дата" value="2026-10-19T09:15:30"`
			if tc.dateMode == metadata.FormDateModeDate {
				want = `<input type="date" name="tp.Lines.0.Дата" value="2026-10-19"`
			}
			if tc.readOnly {
				want = `<input type="text" name="tp.Lines.0.Дата" value="19.10.2026 09:15:30" disabled`
			}
			if !strings.Contains(page, want) {
				i := strings.Index(page, `name="tp.Lines.0.Дата"`)
				ctx := ""
				if i >= 0 {
					ctx = page[max(0, i-120):min(len(page), i+120)]
				}
				t.Fatalf("дата строки не полем даты с исходным временем; вокруг ячейки: %q", ctx)
			}
			if tc.dateMode == metadata.FormDateModeDate && !strings.Contains(page, `data-tp-date-modes=`) {
				t.Fatal("режим даты не передан в разметку табличной части")
			}
			if tc.readOnly {
				return
			}

			if tc.dateMode == metadata.FormDateModeDate {
				body.Set("tp.Lines.0.Дата", "2026-10-19")
			}
			// Пересохранение формы тем значением, что она показала, дату не трогает.
			rec = httptest.NewRecorder()
			srv.submitEdit(rec, reqWithChi(http.MethodPost, "/ui/catalog/"+ent.Name+"/"+id.String(), body,
				map[string]string{"entity": ent.Name, "id": id.String()}))
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("пересохранение: code=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(open(), want) {
				t.Fatal("после пересохранения дата строки изменилась")
			}
			if tc.dateMode == metadata.FormDateModeDate {
				rows, err := srv.store.GetTablePartRows(ctx, ent.Name, tp.Name, id, tp)
				if err != nil {
					t.Fatal(err)
				}
				got, ok := rows[0]["Дата"].(time.Time)
				if !ok || got.In(time.Local).Format("2006-01-02T15:04:05") != "2026-10-19T00:00:00" {
					t.Fatalf("date-only строка = %v, ожидалась местная полночь", rows[0]["Дата"])
				}
			}
		})
	}
}

func TestTPDateInputValue_KeepsWallClockAndConvertsZoned(t *testing.T) {
	for in, want := range map[string]string{
		"2026-10-19T09:15:30": "2026-10-19T09:15:30", // уже стенные часы — не сдвигать
		"2026-10-19":          "2026-10-19T00:00:00",
		"не дата":             "",
		"":                    "",
	} {
		if got := tpDateInputValue(in); got != want {
			t.Errorf("tpDateInputValue(%q) = %q, want %q", in, got, want)
		}
	}
}
