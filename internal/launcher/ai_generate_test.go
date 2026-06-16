package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/llm"
)

const validCatalogYAML = "name: Клиент\nfields:\n  - {name: Наименование, type: string}\n"

func newTestGenSession(t *testing.T) *genSession {
	t.Helper()
	src := t.TempDir()
	g, err := newGenSession(src)
	if err != nil {
		t.Fatalf("newGenSession: %v", err)
	}
	t.Cleanup(g.close)
	return g
}

func TestGenCreateObject_WritesToOverlay(t *testing.T) {
	g := newTestGenSession(t)
	if err := g.createObject("справочник", "Клиент", validCatalogYAML); err != nil {
		t.Fatalf("createObject: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(g.overlay, "catalogs", "клиент.yaml"))
	if err != nil {
		t.Fatalf("файл не создан в overlay: %v", err)
	}
	if string(got) != validCatalogYAML {
		t.Errorf("содержимое не совпало: %q", got)
	}
	if _, err := os.Stat(filepath.Join(g.srcDir, "catalogs", "клиент.yaml")); !os.IsNotExist(err) {
		t.Error("исходный srcDir не должен меняться")
	}
}

func TestGenCreateObject_UnknownKind(t *testing.T) {
	g := newTestGenSession(t)
	if err := g.createObject("ракета", "X", "name: X\n"); err == nil {
		t.Error("ожидалась ошибка для неизвестного типа")
	}
}

func TestGenCreateObject_BadName(t *testing.T) {
	g := newTestGenSession(t)
	for _, bad := range []string{"", "../evil", "a/b", "a\\b"} {
		if err := g.createObject("справочник", bad, "name: X\n"); err == nil {
			t.Errorf("ожидалась ошибка для имени %q", bad)
		}
	}
}

func TestGenCheck_ReportsBadYAML(t *testing.T) {
	g := newTestGenSession(t)
	if err := g.createObject("документ", "Заявка", "name: Заявка\nfields: [oops"); err != nil {
		t.Fatalf("createObject: %v", err)
	}
	out := g.check()
	if !strings.Contains(out, "Заявка") {
		t.Errorf("check не сообщил об ошибке битого документа: %s", out)
	}
}

func TestGenCheck_CleanIsOK(t *testing.T) {
	g := newTestGenSession(t)
	if err := g.createObject("справочник", "Клиент", validCatalogYAML); err != nil {
		t.Fatalf("createObject: %v", err)
	}
	if out := g.check(); !strings.Contains(strings.ToLower(out), "нет ошибок") {
		t.Errorf("ожидалось «нет ошибок», получено: %s", out)
	}
}

func TestGenDiff_ListsNew(t *testing.T) {
	g := newTestGenSession(t)
	if err := g.createObject("справочник", "Клиент", validCatalogYAML); err != nil {
		t.Fatalf("createObject: %v", err)
	}
	d := g.diff()
	if len(d) != 1 || d[0].Path != "catalogs/клиент.yaml" || d[0].Kind != "новый" || d[0].NewContent != validCatalogYAML {
		t.Fatalf("diff неверный: %+v", d)
	}
}

func TestGenShowObject_ReadsExisting(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "catalogs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "catalogs", "товар.yaml"), []byte("name: Товар\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := newGenSession(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.close)
	if out := g.showObject("Товар"); !strings.Contains(out, "name: Товар") {
		t.Errorf("showObject не вернул YAML: %q", out)
	}
}

func TestGenTools_Dispatch(t *testing.T) {
	g := newTestGenSession(t)
	tools, exec := g.tools()
	if len(tools) != 3 {
		t.Fatalf("ожидалось 3 инструмента, получено %d", len(tools))
	}
	res := exec(context.Background(), llm.ToolCall{
		ID:    "1",
		Name:  "создать_объект",
		Input: map[string]any{"тип": "справочник", "имя": "Клиент", "yaml": validCatalogYAML},
	})
	if res.IsError {
		t.Fatalf("создать_объект вернул ошибку: %s", res.Content)
	}
	if _, err := os.Stat(filepath.Join(g.overlay, "catalogs", "клиент.yaml")); err != nil {
		t.Errorf("инструмент не записал объект: %v", err)
	}
	chk := exec(context.Background(), llm.ToolCall{ID: "2", Name: "проверить_конфигурацию", Input: map[string]any{}})
	if chk.IsError {
		t.Errorf("проверить_конфигурацию не должен быть ошибкой: %s", chk.Content)
	}
}

func TestGenerateSystemPrompt_HasMetadataFormat(t *testing.T) {
	for _, want := range []string{"tableparts", "reference:", "type: number", "posting: true"} {
		if !strings.Contains(aiGenerateSystem, want) {
			t.Errorf("системный промпт генератора не содержит %q", want)
		}
	}
}
