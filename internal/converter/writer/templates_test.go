package writer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/printform"
)

func writeTemplateSrc(t *testing.T, sourceDir, kind, owner, name string) {
	t.Helper()
	ext := filepath.Join(sourceDir, kind, owner, "Templates", name, "Ext")
	if err := os.MkdirAll(ext, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ext, "Template.mxl"), []byte("mxl"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// Макеты обработок шли в printforms/ с document:<Обработка>, но printform
// привязывается только к документам/справочникам — привязка была мёртвой
// (issue #48 п.4). Теперь они становятся src/<имя>.proc.layout.yaml.
func TestWriteTemplatesProcessorLayouts(t *testing.T) {
	src := t.TempDir()
	out := t.TempDir()
	writeTemplateSrc(t, src, "DataProcessors", "ЗагрузкаКурсов", "Заголовок")
	writeTemplateSrc(t, src, "DataProcessors", "ЗагрузкаКурсов", "Строка")
	writeTemplateSrc(t, src, "Documents", "Реализация", "Накладная")

	rep := &ConversionReport{}
	if err := WriteTemplates(src, out, rep); err != nil {
		t.Fatalf("WriteTemplates: %v", err)
	}

	// Макеты обработки — один layout с двумя областями.
	layoutPath := filepath.Join(out, "src", "загрузкакурсов.proc.layout.yaml")
	lt, err := printform.LoadLayout(layoutPath)
	if err != nil {
		t.Fatalf("layout не создан или не парсится: %v", err)
	}
	if len(lt.Areas) != 2 {
		t.Fatalf("ожидались 2 области, получено %d: %+v", len(lt.Areas), lt.Areas)
	}
	for _, area := range []string{"Заголовок", "Строка"} {
		if _, ok := lt.Areas[area]; !ok {
			t.Errorf("нет области %q", area)
		}
	}
	// Исходники скопированы рядом.
	if _, err := os.Stat(filepath.Join(out, "src", "загрузкакурсов_заголовок.src.mxl")); err != nil {
		t.Errorf("исходник макета не скопирован: %v", err)
	}
	// В printforms/ макеты обработки НЕ попали, а макет документа — попал.
	if _, err := os.Stat(filepath.Join(out, "printforms", "загрузкакурсов_заголовок.yaml")); err == nil {
		t.Errorf("макет обработки не должен попадать в printforms/")
	}
	if _, err := os.Stat(filepath.Join(out, "printforms", "реализация_накладная.yaml")); err != nil {
		t.Errorf("макет документа должен остаться в printforms/: %v", err)
	}
	// Отчёт упоминает layout-заготовку.
	if len(rep.ProcessorLayouts) != 1 || !strings.Contains(rep.String(), "загрузкакурсов.proc.layout.yaml") {
		t.Errorf("отчёт не упоминает layout: %+v", rep.ProcessorLayouts)
	}
}
