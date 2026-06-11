package extform

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
)

const sampleProc = `name: ТестоваяОбработка
title: Тестовая обработка
params:
  - name: Режим
    type: string
code: |
  // Обработка
  Процедура Выполнить()
    Сообщить("привет");
  КонецПроцедуры
`

func newProcRepo(t *testing.T) (*ProcessorRepo, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	r := NewProcessors(db)
	if err := r.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	return r, ctx
}

func TestProcessorRepo_CRUDAndLoad(t *testing.T) {
	r, ctx := newProcRepo(t)

	rec := &ProcessorRecord{Name: "ТестоваяОбработка", Content: []byte(sampleProc), Author: "Иван", Version: "1.0.0", UploadedBy: "admin"}
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
	// По умолчанию недоверенная.
	if all[0].Trusted {
		t.Error("новая обработка должна быть недоверенной")
	}

	// LoadEnabled парсит метаданные и код.
	procs, programs, err := r.LoadEnabled(ctx)
	if err != nil || len(procs) != 1 {
		t.Fatalf("LoadEnabled: got %d err=%v", len(procs), err)
	}
	if procs[0].Name != "ТестоваяОбработка" || len(procs[0].Params) != 1 {
		t.Errorf("метаданные разобраны неверно: %+v", procs[0])
	}
	prog := programs["ТестоваяОбработка"]
	if prog == nil || !hasProc(prog, "Выполнить") {
		t.Error("код не разобран или нет Выполнить")
	}

	// Доверие.
	if err := r.SetTrusted(ctx, rec.ID, true); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Get(ctx, rec.ID)
	if !got.Trusted {
		t.Error("SetTrusted не сработал")
	}

	if err := r.Delete(ctx, rec.ID); err != nil {
		t.Fatal(err)
	}
	if all, _ := r.List(ctx); len(all) != 0 {
		t.Errorf("после Delete пусто, got %d", len(all))
	}
}

func TestParseProcessorUpload(t *testing.T) {
	// голый YAML
	p, err := ParseProcessorUpload([]byte(sampleProc))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "ТестоваяОбработка" {
		t.Errorf("имя: %+v", p)
	}

	// бандл
	bundle := `manifest:
  kind: processor
  name: ТестоваяОбработка
  author: Пётр
  version: 2.0.0
form:
  name: ТестоваяОбработка
  code: |
    Процедура Выполнить()
    КонецПроцедуры
`
	p2, err := ParseProcessorUpload([]byte(bundle))
	if err != nil {
		t.Fatal(err)
	}
	if p2.Author != "Пётр" || p2.Version != "2.0.0" {
		t.Errorf("манифест не разобран: %+v", p2)
	}
	if strings.Contains(string(p2.Content), "manifest") {
		t.Error("Content не должен содержать manifest")
	}

	// нет кода → ошибка
	if _, err := ParseProcessorUpload([]byte("name: Пусто\n")); err == nil {
		t.Error("ожидалась ошибка при отсутствии code")
	}
	// код без Выполнить → ошибка
	if _, err := ParseProcessorUpload([]byte("name: Б\ncode: |\n  Процедура Иное()\n  КонецПроцедуры\n")); err == nil {
		t.Error("ожидалась ошибка при отсутствии Выполнить()")
	}
	// синтаксически битый код → ошибка
	if _, err := ParseProcessorUpload([]byte("name: Б\ncode: |\n  Процедура Выполнить(\n")); err == nil {
		t.Error("ожидалась ошибка компиляции кода")
	}
}

func TestBuildProcessorBundle_RoundTrip(t *testing.T) {
	rec := &ProcessorRecord{Name: "Обр", Content: []byte("name: Обр\ncode: |\n  Процедура Выполнить()\n  КонецПроцедуры\n"), Author: "А", Version: "1.0.0"}
	data, err := BuildProcessorBundle(rec, "0.5.0")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseProcessorUpload(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Обр" || p.Author != "А" || p.Version != "1.0.0" || p.MinPlatform != "0.5.0" {
		t.Errorf("round-trip потерял данные: %+v", p)
	}
}
