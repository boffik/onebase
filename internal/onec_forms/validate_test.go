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

func hasWarnCode(warns []Warning, code string) bool {
	for _, w := range warns {
		if w.Code == code {
			return true
		}
	}
	return false
}

// Команды объявлены, но не размещены кнопкой (kind: Кнопка) — W014 warn.
func TestValidate_CommandNotPlaced(t *testing.T) {
	noBtn := `schema: onebase.form/v1
form:
  name: ФормаОбъекта
  kind: object
  entity: Заявка
commands:
  - name: Принять
    title: { ru: "Принять" }
    action: КомандаПринятьНажатие
elements:
  - kind: ПолеВвода
    name: ПолеНомер
    data_path: Объект.Номер
`
	warns, _ := Validate(writeYAML(t, noBtn))
	if !hasWarnCode(warns, W014_CommandNotPlaced) {
		t.Errorf("W014 не сработал при команде без кнопки: %+v", warns)
	}
	// С размещённой кнопкой — W014 молчит.
	withBtn := strings.Replace(noBtn, "  - kind: ПолеВвода\n    name: ПолеНомер",
		"  - kind: Кнопка\n    name: КомандаПринять\n  - kind: ПолеВвода\n    name: ПолеНомер", 1)
	warns2, _ := Validate(writeYAML(t, withBtn))
	if hasWarnCode(warns2, W014_CommandNotPlaced) {
		t.Errorf("W014 ложно сработал при наличии kind: Кнопка: %+v", warns2)
	}
}

// Form-local реквизит ссылочного типа в ПолеВвода не даёт пикер — W015 warn;
// то же поле объекта (Объект.X) — молчит.
func TestValidate_FormLocalRefField(t *testing.T) {
	local := `schema: onebase.form/v1
form:
  name: ФормаОбъекта
  kind: object
  entity: Заявка
attributes:
  - name: Причина
    type: CatalogRef.ПричинаОтказа
    save: false
elements:
  - kind: ПолеВвода
    name: ПолеПричина
    data_path: Причина
`
	warns, _ := Validate(writeYAML(t, local))
	if !hasWarnCode(warns, W015_FormLocalRefField) {
		t.Errorf("W015 не сработал для form-local reference: %+v", warns)
	}
	obj := strings.Replace(local, "data_path: Причина", "data_path: Объект.Причина", 1)
	warns2, _ := Validate(writeYAML(t, obj))
	if hasWarnCode(warns2, W015_FormLocalRefField) {
		t.Errorf("W015 ложно сработал для поля объекта: %+v", warns2)
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
