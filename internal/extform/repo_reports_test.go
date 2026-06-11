package extform

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
)

const sampleReport = `name: ОстаткиПоСкладам
title: Остатки по складам
params:
  - name: Склад
    type: string
query: |
  ВЫБРАТЬ Номенклатура, Количество
  ИЗ РегистрНакопления.Остатки.Остатки()
`

func newReportRepo(t *testing.T) (*ReportRepo, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	r := NewReports(db)
	if err := r.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	return r, ctx
}

func TestReportRepo_CRUD(t *testing.T) {
	r, ctx := newReportRepo(t)

	rec := &ReportRecord{Name: "ОстаткиПоСкладам", Content: []byte(sampleReport), Author: "Иван", Version: "1.0.0", UploadedBy: "admin"}
	if err := r.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if rec.ID == "" {
		t.Fatal("Save должен проставить ID")
	}

	all, err := r.List(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("List: got %d err=%v", len(all), err)
	}
	if !all[0].Enabled || all[0].Author != "Иван" {
		t.Errorf("неожиданная запись: %+v", all[0])
	}

	reps, err := r.LoadEnabledReports(ctx)
	if err != nil || len(reps) != 1 {
		t.Fatalf("LoadEnabledReports: got %d err=%v", len(reps), err)
	}
	if reps[0].Name != "ОстаткиПоСкладам" || reps[0].Title != "Остатки по складам" {
		t.Errorf("отчёт распарсился неверно: %+v", reps[0])
	}
	if len(reps[0].Params) != 1 || reps[0].Params[0].Name != "Склад" {
		t.Errorf("параметры не разобрались: %+v", reps[0].Params)
	}

	if err := r.SetEnabled(ctx, rec.ID, false); err != nil {
		t.Fatal(err)
	}
	if en, _ := r.ListEnabled(ctx); len(en) != 0 {
		t.Errorf("после выключения ListEnabled пуст, got %d", len(en))
	}

	if err := r.Delete(ctx, rec.ID); err != nil {
		t.Fatal(err)
	}
	if all, _ := r.List(ctx); len(all) != 0 {
		t.Errorf("после Delete пусто, got %d", len(all))
	}
}

func TestParseReportUpload(t *testing.T) {
	// голый YAML
	p, err := ParseReportUpload([]byte(sampleReport))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "ОстаткиПоСкладам" {
		t.Errorf("имя из отчёта: %+v", p)
	}

	// бандл
	bundle := `manifest:
  kind: report
  name: ОстаткиПоСкладам
  author: Пётр
  version: 3.0.0
form:
  name: ОстаткиПоСкладам
  query: ВЫБРАТЬ 1
`
	p2, err := ParseReportUpload([]byte(bundle))
	if err != nil {
		t.Fatal(err)
	}
	if p2.Author != "Пётр" || p2.Version != "3.0.0" {
		t.Errorf("манифест не разобран: %+v", p2)
	}
	if strings.Contains(string(p2.Content), "manifest") {
		t.Error("Content не должен содержать manifest")
	}

	// без query — ошибка
	if _, err := ParseReportUpload([]byte("name: Пусто\n")); err == nil {
		t.Error("ожидалась ошибка при пустом query")
	}
}

func TestBuildReportBundle_RoundTrip(t *testing.T) {
	rec := &ReportRecord{Name: "Отчёт", Content: []byte("name: Отчёт\nquery: ВЫБРАТЬ 1\n"), Author: "А", Version: "1.1.0"}
	data, err := BuildReportBundle(rec, "0.5.0")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseReportUpload(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Отчёт" || p.Author != "А" || p.Version != "1.1.0" || p.MinPlatform != "0.5.0" {
		t.Errorf("round-trip потерял данные: %+v", p)
	}
}
