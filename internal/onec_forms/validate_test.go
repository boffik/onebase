package onec_forms

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeYAML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.form.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Корректная форма не должна давать error-warnings.
func TestValidate_OK(t *testing.T) {
	yamlBody := `schema: onebase.form/v1
form:
  name: ФормаОбъекта
  kind: object
  entity: Контрагент
elements:
  - kind: ГруппаФормы
    name: Шапка
    children:
      - kind: ПолеВвода
        name: ПолеНаименование
        data_path: Объект.Наименование
`
	warns, err := Validate(writeYAML(t, yamlBody))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warns {
		if w.Severity == SeverityError {
			t.Errorf("неожиданный error: %s", w)
		}
	}
}

// Отсутствие data_path у ПолеВвода — это error W012.
func TestValidate_MissingDataPath(t *testing.T) {
	yamlBody := `schema: onebase.form/v1
form:
  name: F
  entity: E
elements:
  - kind: ПолеВвода
    name: ПолеБезПути
`
	warns, _ := Validate(writeYAML(t, yamlBody))
	hasW012 := false
	for _, w := range warns {
		if w.Code == W012_MissingDataPath && w.Severity == SeverityError {
			hasW012 = true
		}
	}
	if !hasW012 {
		t.Errorf("W012 не сработал: %+v", warns)
	}
}

// Неизвестный kind — W010 warn (не error, можно «протолкнуть»).
func TestValidate_UnknownKind(t *testing.T) {
	yamlBody := `schema: onebase.form/v1
form:
  name: F
  entity: E
elements:
  - kind: КакаяТоЭкзотика
    name: X
`
	warns, _ := Validate(writeYAML(t, yamlBody))
	hasW010 := false
	for _, w := range warns {
		if w.Code == W010_UnknownElement && w.Severity == SeverityWarn {
			hasW010 = true
		}
	}
	if !hasW010 {
		t.Errorf("W010 не сработал: %+v", warns)
	}
}

// Реквизит без type — error W022.
func TestValidate_AttributeWithoutType(t *testing.T) {
	yamlBody := `schema: onebase.form/v1
form:
  name: F
  entity: E
attributes:
  - name: Объект
`
	warns, _ := Validate(writeYAML(t, yamlBody))
	hasErr := false
	for _, w := range warns {
		if w.Code == W022_UnknownType && w.Severity == SeverityError {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("W022 не сработал: %+v", warns)
	}
}

// Несуществующий файл — возвращает W003 (invalid yaml / file).
func TestValidate_MissingFile(t *testing.T) {
	warns, _ := Validate(filepath.Join(t.TempDir(), "nope.form.yaml"))
	if len(warns) == 0 {
		t.Fatal("ожидался хотя бы один warning об отсутствующем файле")
	}
	if warns[0].Code != W003_InvalidYAML {
		t.Errorf("ожидался W003, получен %s", warns[0].Code)
	}
}

// Дубликат имени — info-warning W050 (не блокирует, но подсвечивает).
func TestValidate_DuplicateNames(t *testing.T) {
	yamlBody := `schema: onebase.form/v1
form:
  name: F
  entity: E
elements:
  - kind: ГруппаФормы
    name: Группа1
    children:
      - kind: ПолеВвода
        name: Поле
        data_path: Объект.A
  - kind: ГруппаФормы
    name: Группа2
    children:
      - kind: ПолеВвода
        name: Поле
        data_path: Объект.B
`
	warns, _ := Validate(writeYAML(t, yamlBody))
	hasDup := false
	for _, w := range warns {
		if w.Code == W050_NeedsReview && strings.Contains(w.Message, "имя встречается у нескольких") {
			hasDup = true
		}
	}
	if !hasDup {
		t.Errorf("дубликат имени не обнаружен: %+v", warns)
	}
}
