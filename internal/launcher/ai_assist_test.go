package launcher

import (
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/report"
)

// projectSchemaText строит срез из загруженного проекта — проверяем на литерале.
func TestProjectSchemaText(t *testing.T) {
	proj := &project.Project{
		Entities: []*metadata.Entity{{
			Name: "Заявка", Kind: metadata.KindDocument, Posting: true,
			Fields: []metadata.Field{{Name: "Дата", Type: metadata.FieldTypeDate}},
			TableParts: []metadata.TablePart{
				{Name: "Строки", Fields: []metadata.Field{{Name: "Товар", Type: metadata.FieldTypeString}}},
			},
		}},
		Reports: []*report.Report{{Name: "Сводка", Title: "Сводный отчёт"}},
	}
	txt := projectSchemaText(proj)
	for _, sub := range []string{"Заявка", "(проводится)", "ТЧ Строки", "Сводка — Сводный отчёт"} {
		if !strings.Contains(txt, sub) {
			t.Fatalf("в срезе нет %q: %s", sub, txt)
		}
	}
}
