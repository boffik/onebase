package launcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/configdb"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/printform"
	"github.com/ivantit66/onebase/internal/storage"
)

// План 64, этап 5a (6.4): создание макета с нуля.

func TestValidLayoutName(t *testing.T) {
	ok := []string{"Накладная", "счёт-фактура", "form1"}
	bad := []string{"", "  ", "../evil", "a/b", `a\b`, "x..y"}
	for _, n := range ok {
		if !validLayoutName(n) {
			t.Errorf("validLayoutName(%q) = false, want true", n)
		}
	}
	for _, n := range bad {
		if validLayoutName(n) {
			t.Errorf("validLayoutName(%q) = true, want false", n)
		}
	}
}

// Скелет для сущности с ТЧ парсится как валидный v2-макет: 4 области по порядку,
// columns по числу полей ТЧ, binding.repeat связывает «Строка» с первой ТЧ.
func TestEntityLayoutSkeleton_RoundTrip(t *testing.T) {
	ent := &metadata.Entity{
		Name: "РеализацияТоваров",
		Kind: metadata.KindDocument,
		TableParts: []metadata.TablePart{{
			Name: "Товары",
			Fields: []metadata.Field{
				{Name: "Номенклатура", Type: metadata.FieldTypeString},
				{Name: "Количество", Type: metadata.FieldTypeNumber},
				{Name: "Сумма", Type: metadata.FieldTypeNumber},
			},
		}},
	}
	lt := buildEntityLayoutSkeleton("ТоварнаяНакладная", ent)
	src, err := marshalLayout(lt)
	if err != nil {
		t.Fatalf("marshalLayout: %v", err)
	}
	// areas должны быть sequence (v2), не mapping.
	if !strings.Contains(string(src), "areas:") {
		t.Fatalf("в YAML нет areas:\n%s", src)
	}

	parsed, err := printform.ParseLayoutBytes(src)
	if err != nil {
		t.Fatalf("ParseLayoutBytes: %v\n%s", err, src)
	}
	wantAreas := []string{"Заголовок", "ШапкаТаблицы", "Строка", "Итоги"}
	if len(parsed.Areas) != len(wantAreas) {
		t.Fatalf("областей %d, want %d", len(parsed.Areas), len(wantAreas))
	}
	for i, want := range wantAreas {
		if parsed.Areas[i].Name != want {
			t.Errorf("область[%d] = %q, want %q", i, parsed.Areas[i].Name, want)
		}
	}
	// columns по числу полей ТЧ (3).
	if len(parsed.Columns) != 3 {
		t.Errorf("columns %d, want 3", len(parsed.Columns))
	}
	// binding.repeat → Строка ← Товары.
	if parsed.Binding == nil || len(parsed.Binding.Repeat) != 1 {
		t.Fatalf("binding.repeat не задан: %+v", parsed.Binding)
	}
	rb := parsed.Binding.Repeat[0]
	if rb.Area != "Строка" || rb.Source != "Товары" {
		t.Errorf("repeat = {%q,%q}, want {Строка,Товары}", rb.Area, rb.Source)
	}
	// заголовок — bold по центру с именем сущности.
	hdr := parsed.Area("Заголовок")
	if hdr == nil || len(hdr.Rows) == 0 || len(hdr.Rows[0].Cells) == 0 {
		t.Fatal("область Заголовок пуста")
	}
	hc := hdr.Rows[0].Cells[0]
	if hc.Text != "РеализацияТоваров" || !hc.Bold || hc.Align != "center" {
		t.Errorf("ячейка заголовка = %+v", hc)
	}
}

// Без ТЧ — columns по умолчанию 3, binding пуст.
func TestEntityLayoutSkeleton_NoTablePart(t *testing.T) {
	ent := &metadata.Entity{Name: "Контрагент", Kind: metadata.KindCatalog}
	lt := buildEntityLayoutSkeleton("Карточка", ent)
	if len(lt.Columns) != 3 {
		t.Errorf("columns %d, want 3", len(lt.Columns))
	}
	if lt.Binding != nil {
		t.Errorf("binding должен быть nil без ТЧ, got %+v", lt.Binding)
	}
}

// Пустой скелет для .os-формы — одна область «Макет» 3×3.
func TestEmptyLayoutSkeleton_RoundTrip(t *testing.T) {
	lt := buildEmptyLayoutSkeleton("МойМакет")
	src, err := marshalLayout(lt)
	if err != nil {
		t.Fatalf("marshalLayout: %v", err)
	}
	parsed, err := printform.ParseLayoutBytes(src)
	if err != nil {
		t.Fatalf("ParseLayoutBytes: %v\n%s", err, src)
	}
	if len(parsed.Areas) != 1 || parsed.Areas[0].Name != "Макет" {
		t.Fatalf("ожидалась одна область «Макет», got %+v", parsed.Areas)
	}
	if len(parsed.Areas[0].Rows) != 3 {
		t.Errorf("строк %d, want 3", len(parsed.Areas[0].Rows))
	}
	for i, row := range parsed.Areas[0].Rows {
		if len(row.Cells) != 3 {
			t.Errorf("строка %d: ячеек %d, want 3", i, len(row.Cells))
		}
	}
}

// newLayoutTestBase создаёт file-mode базу с проектом в temp-каталоге и сущностью.
func newLayoutTestBase(t *testing.T) (*handler, *Base, string) {
	t.Helper()
	s := newTestStore(t)
	dir := t.TempDir()
	// минимальный проект: документ с табличной частью.
	if err := os.MkdirAll(filepath.Join(dir, "documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "name: Реализация\nfields:\n  - name: Номер\n    type: string\n  - name: Дата\n    type: date\ntableparts:\n  - name: Товары\n    fields:\n      - name: Номенклатура\n        type: string\n      - name: Сумма\n        type: number\n"
	if err := os.WriteFile(filepath.Join(dir, "documents", "реализация.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &Base{Name: "Тест", ConfigSource: "file", Path: dir}
	if err := s.Add(b); err != nil {
		t.Fatalf("Add: %v", err)
	}
	return &handler{store: s, runner: NewRunner()}, b, dir
}

func postNewLayout(t *testing.T, h *handler, b *Base, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", b.ID)
	req := httptest.NewRequest(http.MethodPost, "/bases/"+b.ID+"/configurator/new-layout",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Onebase-Ajax", "1") // получить JSON вместо HTML
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.configuratorNewLayout(rec, req)
	return rec
}

// Happy-path: создание макета у сущности пишет parseable v2-файл на диск.
func TestNewLayout_EntityHappyPath(t *testing.T) {
	h, b, dir := newLayoutTestBase(t)
	rec := postNewLayout(t, h, b, url.Values{
		"entity": {"Реализация"},
		"name":   {"ТоварнаяНакладная"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, тело %s", rec.Code, rec.Body.String())
	}
	path := filepath.Join(dir, "printforms", "ТоварнаяНакладная.layout.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("файл макета не создан: %v", err)
	}
	parsed, err := printform.ParseLayoutBytes(data)
	if err != nil {
		t.Fatalf("созданный макет не парсится: %v", err)
	}
	if parsed.Area("Строка") == nil {
		t.Error("в макете нет области Строка")
	}
	// binding по ТЧ Товары.
	if parsed.Binding == nil || len(parsed.Binding.Repeat) != 1 || parsed.Binding.Repeat[0].Source != "Товары" {
		t.Errorf("binding по ТЧ Товары не задан: %+v", parsed.Binding)
	}
}

// Отказ на дубликат.
func TestNewLayout_DuplicateRejected(t *testing.T) {
	h, b, dir := newLayoutTestBase(t)
	if err := os.MkdirAll(filepath.Join(dir, "printforms"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "printforms", "Дубль.layout.yaml")
	if err := os.WriteFile(existing, []byte("areas: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.ReadFile(existing)

	rec := postNewLayout(t, h, b, url.Values{
		"entity": {"Реализация"},
		"name":   {"Дубль"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "уже существует") && !strings.Contains(rec.Body.String(), "exists") {
		t.Errorf("ожидалась ошибка дубликата, тело: %s", rec.Body.String())
	}
	after, _ := os.ReadFile(existing)
	if string(after) != string(orig) {
		t.Error("существующий файл был перезаписан")
	}
}

// Отказ на плохое имя (../).
func TestNewLayout_BadNameRejected(t *testing.T) {
	h, b, dir := newLayoutTestBase(t)
	rec := postNewLayout(t, h, b, url.Values{
		"entity": {"Реализация"},
		"name":   {"../evil"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Недопустимое") && !strings.Contains(rec.Body.String(), "Invalid") {
		t.Errorf("ожидалась ошибка имени, тело: %s", rec.Body.String())
	}
	// никакого файла за пределами printforms не создано.
	if _, err := os.Stat(filepath.Join(dir, "evil.layout.yaml")); err == nil {
		t.Error("создан файл по traversal-пути")
	}
}

// newLayoutTestBaseDB создаёт database-config базу: SQLite-файл с таблицей
// _onebase_config, в которую импортирован минимальный проект (документ с ТЧ).
// Возвращает handler, базу и путь к db-файлу. Долг этапа 5a — configdb-режим.
func newLayoutTestBaseDB(t *testing.T) (*handler, *Base) {
	t.Helper()
	s := newTestStore(t)
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "cfg.db")

	// исходный каталог конфигурации для импорта в БД.
	if err := os.MkdirAll(filepath.Join(dir, "documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "name: Реализация\nfields:\n  - name: Номер\n    type: string\ntableparts:\n  - name: Товары\n    fields:\n      - name: Номенклатура\n        type: string\n      - name: Сумма\n        type: number\n"
	if err := os.WriteFile(filepath.Join(dir, "documents", "реализация.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	repo := configdb.New(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		db.Close()
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := repo.ImportFromDir(ctx, dir); err != nil {
		db.Close()
		t.Fatalf("ImportFromDir: %v", err)
	}
	db.Close()

	b := &Base{Name: "ТестБД", ConfigSource: "database", DBType: "sqlite", DBPath: dbPath}
	if err := s.Add(b); err != nil {
		t.Fatalf("Add: %v", err)
	}
	return &handler{store: s, runner: NewRunner()}, b
}

// configReadLayout читает содержимое макета из _onebase_config.
func configReadLayout(t *testing.T, b *Base, relPath string) ([]byte, bool) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, b.DBPath)
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer db.Close()
	content, ok, err := configdb.New(db).ReadFile(ctx, relPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return content, ok
}

// configdb happy-path: создание макета у сущности пишет parseable v2 в _onebase_config.
func TestNewLayout_DBHappyPath(t *testing.T) {
	h, b := newLayoutTestBaseDB(t)
	rec := postNewLayout(t, h, b, url.Values{
		"entity": {"Реализация"},
		"name":   {"ТоварнаяНакладная"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, тело %s", rec.Code, rec.Body.String())
	}
	content, ok := configReadLayout(t, b, "printforms/ТоварнаяНакладная.layout.yaml")
	if !ok {
		t.Fatal("макет не записан в _onebase_config")
	}
	parsed, err := printform.ParseLayoutBytes(content)
	if err != nil {
		t.Fatalf("созданный макет не парсится: %v\n%s", err, content)
	}
	if parsed.Area("Строка") == nil {
		t.Error("в макете нет области Строка")
	}
	if parsed.Binding == nil || len(parsed.Binding.Repeat) != 1 || parsed.Binding.Repeat[0].Source != "Товары" {
		t.Errorf("binding по ТЧ Товары не задан: %+v", parsed.Binding)
	}
}

// configdb duplicate: повторное создание того же макета отклоняется, исходное не меняется.
func TestNewLayout_DBDuplicateRejected(t *testing.T) {
	h, b := newLayoutTestBaseDB(t)
	// первый раз — успех.
	rec1 := postNewLayout(t, h, b, url.Values{"entity": {"Реализация"}, "name": {"Дубль"}})
	if rec1.Code != http.StatusOK {
		t.Fatalf("первый POST код %d", rec1.Code)
	}
	before, ok := configReadLayout(t, b, "printforms/Дубль.layout.yaml")
	if !ok {
		t.Fatal("первый макет не записан")
	}

	// второй раз — отказ.
	rec2 := postNewLayout(t, h, b, url.Values{"entity": {"Реализация"}, "name": {"Дубль"}})
	if rec2.Code != http.StatusOK {
		t.Fatalf("второй POST код %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "уже существует") && !strings.Contains(rec2.Body.String(), "exists") {
		t.Errorf("ожидалась ошибка дубликата, тело: %s", rec2.Body.String())
	}
	after, _ := configReadLayout(t, b, "printforms/Дубль.layout.yaml")
	if string(after) != string(before) {
		t.Error("существующая запись макета была перезаписана")
	}
}

// .os-форма: создаётся пустой парный макет 3×3.
func TestNewLayout_OSForm(t *testing.T) {
	h, b, dir := newLayoutTestBase(t)
	if err := os.MkdirAll(filepath.Join(dir, "printforms"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "printforms", "ПечатьЧека.os"), []byte("// форма\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := postNewLayout(t, h, b, url.Values{"osform": {"ПечатьЧека"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d, тело %s", rec.Code, rec.Body.String())
	}
	path := filepath.Join(dir, "printforms", "ПечатьЧека.layout.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("парный макет не создан: %v", err)
	}
	parsed, err := printform.ParseLayoutBytes(data)
	if err != nil {
		t.Fatalf("макет не парсится: %v", err)
	}
	if len(parsed.Areas) != 1 || parsed.Areas[0].Name != "Макет" {
		t.Errorf("ожидалась область «Макет», got %+v", parsed.Areas)
	}
}
