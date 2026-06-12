package configcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDSL_Broken(t *testing.T) {
	src := `Процедура Привет(
    Сообщить("Hi")
КонецПроцедуры`
	issues := ParseDSL(src, "test.os")
	if len(issues) == 0 {
		t.Fatal("expected at least one issue for missing )")
	}
}

func TestCheckWidgetYAML_OK(t *testing.T) {
	yaml := `name: ВыручкаМесяца
type: kpi
title: Выручка
format: money
query: ВЫБРАТЬ СУММА(Сумма) КАК Значение ИЗ Документ.X`
	if issues := CheckWidgetYAML(yaml, "ВыручкаМесяца"); len(issues) != 0 {
		t.Fatalf("expected clean widget, got %+v", issues)
	}
}

func TestCheckWidgetYAML_UnknownType(t *testing.T) {
	yaml := `name: X
type: gauge
title: ok`
	issues := CheckWidgetYAML(yaml, "X")
	if len(issues) == 0 {
		t.Fatal("expected error on unknown widget type")
	}
	if !strings.Contains(issues[0].Message, "type") && !strings.Contains(issues[0].Message, "тип") {
		t.Errorf("expected type-related message, got %q", issues[0].Message)
	}
}

func TestCheckHomePageYAML_Empty(t *testing.T) {
	if issues := CheckHomePageYAML(""); len(issues) != 0 {
		t.Fatalf("empty body should be considered valid, got %+v", issues)
	}
}

func TestCheckHomePageYAML_Bad(t *testing.T) {
	bad := "title: Главная\nlayout: ::not-yaml::\n  - broken"
	if issues := CheckHomePageYAML(bad); len(issues) == 0 {
		t.Fatal("expected YAML parse error")
	}
}

func TestCheckDir_WithWidget(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, filepath.Join(dir, "widgets", "ok.yaml"), `name: A
type: kpi
title: A
query: SELECT 1`)
	mkFile(t, filepath.Join(dir, "src", "broken.os"), `Процедура X(
КонецПроцедуры`)
	mkFile(t, filepath.Join(dir, "src", "good.os"), `Процедура Y()
КонецПроцедуры`)

	issues := CheckDir(dir)
	var hasBroken bool
	for _, i := range issues {
		if strings.Contains(i.File, "broken.os") {
			hasBroken = true
		}
	}
	if !hasBroken {
		t.Fatalf("expected broken.os issue, got: %+v", issues)
	}
}

// CheckDir должен валидировать журналы/роли/печатные формы пофайлово, с
// указанием конкретного файла (раньше эти типы проверял только project.Load,
// который падал на первой ошибке без локации).
func TestCheckDir_NewObjectTypes(t *testing.T) {
	dir := t.TempDir()
	// журнал: columns строками вместо структур {field: ...} — роняет парсинг
	mkFile(t, filepath.Join(dir, "journals", "j.yaml"), `name: J
documents: [Док]
columns:
  - Дата
  - Сумма`)
	// роль: documents списком вместо map — невалидный формат прав
	mkFile(t, filepath.Join(dir, "roles", "r.yaml"), `name: R
permissions:
  documents: [a, b, c]`)
	// печатная форма: выдуманный «layout:» — парсится, но форма пустая
	mkFile(t, filepath.Join(dir, "printforms", "p.yaml"), `name: P
document: Док
layout: |
  Область Шапка
    Поле Дата`)
	// корректная legacy печатная форма — валидна, но устарела: ожидаем
	// предупреждение о миграции (план 64, этап 4.6), не ошибку «пустая».
	mkFile(t, filepath.Join(dir, "printforms", "ok.yaml"), `name: OK
document: Док
title: OK
header: "**X**: {{X}}"`)

	issues := CheckDir(dir)
	want := map[string]bool{"journals/j.yaml": false, "roles/r.yaml": false, "printforms/p.yaml": false}
	var okMigrate bool
	for _, i := range issues {
		if _, ok := want[i.File]; ok {
			want[i.File] = true
		}
		if i.File == "printforms/ok.yaml" {
			if !strings.Contains(i.Message, "устаревший формат") {
				t.Errorf("корректная legacy-форма должна давать только предупреждение о миграции: %+v", i)
			}
			okMigrate = true
		}
	}
	if !okMigrate {
		t.Errorf("ожидалось предупреждение о миграции для printforms/ok.yaml. issues=%+v", issues)
	}
	for f, found := range want {
		if !found {
			t.Errorf("ожидалась ошибка для %s, не найдена. issues=%+v", f, issues)
		}
	}
}

func mkFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
