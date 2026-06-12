package extform

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
)

const sampleForm = `name: Накладная-А4
document: РеализацияТоваров
title: Расходная накладная
table:
  source: Товары
  columns:
    - field: Номенклатура
      label: Товар
`

func newRepo(t *testing.T) (*Repo, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	r := New(db)
	if err := r.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	return r, ctx
}

func TestRepo_CRUD(t *testing.T) {
	r, ctx := newRepo(t)

	rec := &Record{
		Document:   "РеализацияТоваров",
		Name:       "Накладная-А4",
		Content:    []byte(sampleForm),
		Author:     "Иван",
		Version:    "1.0.0",
		UploadedBy: "admin",
	}
	if err := r.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if rec.ID == "" {
		t.Fatal("Save должен проставить ID")
	}

	all, err := r.List(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("List: ожидалась 1 запись, got %d, err=%v", len(all), err)
	}
	got := all[0]
	if got.Document != "РеализацияТоваров" || got.Name != "Накладная-А4" || !got.Enabled {
		t.Errorf("неожиданная запись: %+v", got)
	}
	if got.Author != "Иван" || got.Version != "1.0.0" || got.UploadedBy != "admin" {
		t.Errorf("метаданные не сохранились: %+v", got)
	}

	// Включённые legacy-формы парсятся в PrintForm; v2-форм здесь нет.
	pfs, layouts, err := r.LoadEnabledPrintForms(ctx)
	if err != nil || len(pfs) != 1 {
		t.Fatalf("LoadEnabledPrintForms: got %d, err=%v", len(pfs), err)
	}
	if len(layouts) != 0 {
		t.Errorf("v2-форм не ожидалось, got %d", len(layouts))
	}
	if pfs[0].Name != "Накладная-А4" || pfs[0].Title != "Расходная накладная" {
		t.Errorf("форма распарсилась неверно: %+v", pfs[0])
	}

	// Выключение убирает форму из ListEnabled.
	if err := r.SetEnabled(ctx, rec.ID, false); err != nil {
		t.Fatal(err)
	}
	enabled, _ := r.ListEnabled(ctx)
	if len(enabled) != 0 {
		t.Errorf("после выключения ListEnabled должен быть пуст, got %d", len(enabled))
	}

	// Get по id.
	one, err := r.Get(ctx, rec.ID)
	if err != nil || one.Enabled {
		t.Errorf("Get: ожидалась выключенная форма, got %+v, err=%v", one, err)
	}

	// Удаление.
	if err := r.Delete(ctx, rec.ID); err != nil {
		t.Fatal(err)
	}
	all, _ = r.List(ctx)
	if len(all) != 0 {
		t.Errorf("после Delete список должен быть пуст, got %d", len(all))
	}
}

// Внешняя форма в формате макета v2 (top-level areas:) сниффится и попадает в
// список layout-форм, а не legacy (план 64, этап 4.5).
func TestRepo_LoadEnabledPrintForms_SniffsV2(t *testing.T) {
	r, ctx := newRepo(t)
	v2 := `name: НакладнаяV2
document: Реализация
areas:
  - name: Заголовок
    rows:
      - cells:
          - text: "Накладная № {{Номер}}"
            colspan: 1
binding:
  sequence: [Заголовок]
`
	if err := r.Save(ctx, &Record{Document: "Реализация", Name: "НакладнаяV2", Content: []byte(v2)}); err != nil {
		t.Fatal(err)
	}
	// плюс legacy-форма — должна попасть в другой список.
	if err := r.Save(ctx, &Record{Document: "Реализация", Name: "Старая", Content: []byte(sampleForm)}); err != nil {
		t.Fatal(err)
	}

	legacy, layouts, err := r.LoadEnabledPrintForms(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(layouts) != 1 {
		t.Fatalf("ожидалась 1 v2-форма, got %d", len(layouts))
	}
	if layouts[0].Name != "НакладнаяV2" || layouts[0].Layout == nil {
		t.Errorf("v2-форма распарсилась неверно: %+v", layouts[0])
	}
	if layouts[0].Layout.Area("Заголовок") == nil {
		t.Error("в v2-форме потеряна область Заголовок")
	}
	// legacy-форма берёт имя из YAML (name: Накладная-А4), не из имени записи.
	if len(legacy) != 1 || legacy[0].Name != "Накладная-А4" {
		t.Errorf("legacy-форма должна остаться отдельно, got %+v", legacy)
	}
}

// Повторный Save по тому же (document, name) обновляет запись, а не плодит дубль.
func TestRepo_SaveUpsert(t *testing.T) {
	r, ctx := newRepo(t)
	rec := &Record{Document: "Док", Name: "Ф", Content: []byte("name: Ф\ndocument: Док\n")}
	if err := r.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	rec2 := &Record{Document: "Док", Name: "Ф", Content: []byte("name: Ф\ndocument: Док\ntitle: Обновлено\n")}
	if err := r.Save(ctx, rec2); err != nil {
		t.Fatal(err)
	}
	all, _ := r.List(ctx)
	if len(all) != 1 {
		t.Fatalf("ожидалась 1 запись после upsert, got %d", len(all))
	}
	if !strings.Contains(string(all[0].Content), "Обновлено") {
		t.Error("содержимое не обновилось при upsert")
	}
}
