package launcher

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/processor"
	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/report"
)

func testProj() *project.Project {
	return &project.Project{
		Entities: []*metadata.Entity{
			{Name: "Контрагент", Kind: metadata.KindCatalog},
			{Name: "Поступление", Kind: metadata.KindDocument},
		},
		Processors: []*processor.Processor{{Name: "ЗагрузкаЦен"}},
		Reports:    []*report.Report{{Name: "ОСВ"}},
	}
}

// classifyConfigFile раскладывает разбросанные по папкам файлы по (раздел,
// объект, роль), используя индекс модели (issue #132).
func TestClassifyConfigFile(t *testing.T) {
	idx := buildObjectIndex(testProj())
	cases := []struct{ path, cat, obj, label string }{
		{"catalogs/Контрагент.yaml", "Справочники", "Контрагент", "Метаданные"},
		{"src/Контрагент.module.os", "Справочники", "Контрагент", "Модуль объекта"},
		{"forms/контрагент/объекта.form.yaml", "Справочники", "Контрагент", "Форма: объекта"},
		{"forms/контрагент/объекта.form.os", "Справочники", "Контрагент", "Модуль формы: объекта"},
		{"documents/Поступление.yaml", "Документы", "Поступление", "Метаданные"},
		{"src/Поступление.posting.os", "Документы", "Поступление", "Обработка проведения"},
		{"src/ЗагрузкаЦен.proc.os", "Обработки", "ЗагрузкаЦен", "Модуль обработки"},
		{"app.yaml", "Конфигурация", "", "app.yaml"},
		{"config/email.yaml", "Конфигурация", "", "email.yaml"},
		{"src/Неизвестный.module.os", "Прочее", "Неизвестный", "Модуль объекта"}, // нет в индексе → fallback
	}
	for _, c := range cases {
		cat, obj, label := classifyConfigFile(c.path, idx)
		if cat != c.cat || obj != c.obj || label != c.label {
			t.Errorf("classify(%q) = (%q,%q,%q), ожидалось (%q,%q,%q)",
				c.path, cat, obj, label, c.cat, c.obj, c.label)
		}
	}
}

// buildConfigFileTreeFrom собирает разбросанные файлы объекта под один узел,
// упорядочивает разделы как дерево конфигурации, безымянные файлы группирует.
func TestBuildConfigFileTree(t *testing.T) {
	paths := []string{
		"catalogs/Контрагент.yaml",
		"src/Контрагент.module.os",
		"forms/контрагент/объекта.form.yaml",
		"documents/Поступление.yaml",
		"src/Поступление.posting.os",
		"app.yaml",
		"processors/ЗагрузкаЦен.yaml",
		"src/ЗагрузкаЦен.proc.os",
	}
	tree := buildConfigFileTreeFrom(testProj(), paths)

	findCat := func(name string) *fileTreeCategory {
		for i := range tree {
			if tree[i].Name == name {
				return &tree[i]
			}
		}
		return nil
	}
	cat := findCat("Справочники")
	if cat == nil || len(cat.Objects) != 1 {
		t.Fatalf("ожидался раздел «Справочники» с 1 объектом, получили %+v", cat)
	}
	if cat.Objects[0].Name != "Контрагент" || len(cat.Objects[0].Files) != 3 {
		t.Errorf("у «Контрагент» должно быть 3 файла (метаданные+модуль+форма), получили %+v", cat.Objects[0])
	}
	if cat.Objects[0].NodeID != "e-Контрагент" { // фаза 2: id узла для «открыть в редакторе»
		t.Errorf("NodeID = %q, ожидался e-Контрагент", cat.Objects[0].NodeID)
	}
	// Документ: метаданные + проведение под одним объектом.
	if d := findCat("Документы"); d == nil || len(d.Objects) != 1 || len(d.Objects[0].Files) != 2 {
		t.Errorf("ожидался «Документы»→«Поступление» с 2 файлами, получили %+v", d)
	}
	// Конфигурация: безымянная группа (app.yaml).
	cfg := findCat("Конфигурация")
	if cfg == nil || len(cfg.Objects) != 1 || cfg.Objects[0].Name != "" {
		t.Errorf("ожидался «Конфигурация» с безымянной группой, получили %+v", cfg)
	}
	// Порядок разделов: Справочники раньше Документы раньше Обработки раньше Конфигурации.
	rank := map[string]int{}
	for i, c := range tree {
		rank[c.Name] = i
	}
	if !(rank["Справочники"] < rank["Документы"] && rank["Документы"] < rank["Обработки"] && rank["Обработки"] < rank["Конфигурация"]) {
		t.Errorf("неверный порядок разделов: %v", rank)
	}
}

// Вкладка «Файлы» рендерит дерево файлов с просмотрщиком (issue #132).
func TestTabFiles_Render(t *testing.T) {
	data := &configuratorData{
		Base: &Base{ID: "test-base", ConfigSource: "file", Path: "/tmp/x"},
		Tab:  "files",
		Lang: "ru",
		ConfigFileTree: []fileTreeCategory{
			{Name: "Справочники", Objects: []fileTreeObject{
				{Name: "Контрагент", NodeID: "e-Контрагент", Files: []fileTreeFile{
					{Label: "Метаданные", Path: "catalogs/Контрагент.yaml"},
					{Label: "Модуль объекта", Path: "src/Контрагент.module.os"},
				}},
			}},
			{Name: "Конфигурация", Objects: []fileTreeObject{
				{Name: "", Files: []fileTreeFile{{Label: "app.yaml", Path: "app.yaml"}}},
			}},
		},
	}
	var buf bytes.Buffer
	if err := cfgTmpl.ExecuteTemplate(&buf, "tab-files", data); err != nil {
		t.Fatalf("ExecuteTemplate tab-files: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`id="files-tree"`,
		`<summary>Справочники</summary>`,
		`📄 Контрагент`,
		`data-path="catalogs/Контрагент.yaml"`,
		`onclick="cfgViewFile(this)`,
		`/bases/test-base/configurator/file?path=`,
		`class="ftfile loose"`, // безымянная группа (app.yaml)
		`class="ftedit"`,       // фаза 2: «открыть в редакторе»
		`tab=tree`,             // ведёт на вкладку дерева
		`select=`,              // с выбором узла
	} {
		if !strings.Contains(html, want) {
			t.Errorf("tab-files не содержит %q", want)
		}
	}
}
