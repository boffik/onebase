package formdoc

import (
	"strings"
	"testing"
)

// Round-trip через yaml.Node должен сохранять комментарии и порядок ключей —
// иначе двусторонняя синхронизация конструктора форм (issue #164) затирала бы
// ручные правки пользователя. Это центральное требование фундамента.
func TestDoc_RoundTripPreservesCommentsAndOrder(t *testing.T) {
	src := `# Форма звонка — ручной комментарий
schema: onebase.form/v1
form:
  name: ФормаОбъекта   # имя формы
  kind: object
  entity: Звонок
elements:
  - kind: ГруппаФормы
    name: Группа1
    children:
      - kind: ПолеВвода
        name: Поле1
        data_path: Объект.Дата
`
	doc, err := Load([]byte(src))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	out, err := doc.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	got := string(out)

	// Комментарии сохранены.
	if !strings.Contains(got, "# Форма звонка — ручной комментарий") {
		t.Errorf("потерян head-комментарий:\n%s", got)
	}
	if !strings.Contains(got, "# имя формы") {
		t.Errorf("потерян inline-комментарий:\n%s", got)
	}
	// Кириллица в значениях цела.
	if !strings.Contains(got, "data_path: Объект.Дата") {
		t.Errorf("потеряно значение с кириллицей:\n%s", got)
	}
	// Порядок ключей формы сохранён: name → kind → entity.
	iName, iKind, iEntity := strings.Index(got, "name:"), strings.Index(got, "kind:"), strings.Index(got, "entity:")
	if !(iName >= 0 && iName < iKind && iKind < iEntity) {
		t.Errorf("порядок ключей формы нарушен (name=%d kind=%d entity=%d):\n%s", iName, iKind, iEntity, got)
	}
}
