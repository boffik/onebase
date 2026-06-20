package ui

import (
	"testing"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
)

func newRegistryForEnumTest(t *testing.T) *runtime.Registry {
	t.Helper()
	reg := runtime.NewRegistry()
	reg.Load(runtime.LoadOptions{
		Enums: []*metadata.Enum{
			{
				Name:   "Приоритет",
				Values: []string{"Высокий", "Низкий"},
				ValueTitles: map[string]map[string]string{
					"Высокий": {"en": "High"},
				},
			},
		},
	})
	return reg
}

func TestBuildEnumLabels(t *testing.T) {
	reg := newRegistryForEnumTest(t)
	s := &Server{reg: reg}
	ent := &metadata.Entity{
		Name: "Задача",
		Fields: []metadata.Field{
			{Name: "Приоритет", Type: "enum:Приоритет", EnumName: "Приоритет"},
			{Name: "Имя", Type: "string"},
		},
	}
	labels := s.buildEnumLabels(ent, "en")
	if labels["Приоритет"]["Высокий"] != "High" {
		t.Errorf("labels = %v", labels)
	}
	if _, ok := labels["Имя"]; ok {
		t.Error("не-enum поле не должно попадать в EnumLabels")
	}
}

func TestLoadEnumOptions_TranslatesLabels(t *testing.T) {
	reg := newRegistryForEnumTest(t)
	s := &Server{reg: reg}
	ent := &metadata.Entity{
		Name: "Задача",
		Fields: []metadata.Field{
			{Name: "Приоритет", Type: "enum:Приоритет", EnumName: "Приоритет"},
		},
	}
	opts := s.loadEnumOptions(ent, "en")
	got := opts["Приоритет"]
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 опции, %d", len(got))
	}
	if got[0].Value != "Высокий" || got[0].Label != "High" {
		t.Errorf("opt0 = %+v (ждали Value=Высокий Label=High)", got[0])
	}
	if got[1].Value != "Низкий" || got[1].Label != "Низкий" {
		t.Errorf("opt1 = %+v", got[1])
	}
}
