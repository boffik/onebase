package ui

import (
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/metadata"
)

// managedObjectForm — управляемая форма объекта. Важно: без неё механизм
// частичной записи выключен (гейт pickManagedForm), и тест ничего не проверяет.
func managedObjectForm(els ...*metadata.FormElement) *metadata.FormModule {
	return &metadata.FormModule{
		Name: "ФормаОбъекта", Kind: "object",
		LayoutKind: metadata.FormLayoutManaged,
		Elements:   els,
	}
}

func fieldEl(name, dataPath string) *metadata.FormElement {
	return &metadata.FormElement{Kind: metadata.FormElementField, Name: name, DataPath: dataPath}
}

// partialWriteEntity — справочник с полями всех значимых типов и managed-формой,
// показывающей ТОЛЬКО Наименование.
func partialWriteEntity() *metadata.Entity {
	e := &metadata.Entity{
		Name: "Клиент", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Комментарий", Type: metadata.FieldTypeString},
			{Name: "ДатаЗакрытия", Type: metadata.FieldTypeDate},
			{Name: "Активен", Type: metadata.FieldTypeBool},
			{Name: "Сумма", Type: metadata.FieldTypeNumber},
		},
	}
	e.Forms = []*metadata.FormModule{managedObjectForm(fieldEl("ПолеНаим", "Объект.Наименование"))}
	return e
}

// Поля сущности, не размещённые на управляемой форме, переживают запись.
// Это ровно дефект поставки: examples/crm КонтактноеЛицо терял Наименование,
// Сделка — ДатаЗакрытия, examples/tasks Задача — три поля.
func TestSubmitEdit_ManagedForm_UnplacedFieldsPreserved(t *testing.T) {
	ent := partialWriteEntity()
	s, ctx := newSubmitTestServer(t, []*metadata.Entity{ent})

	id := uuid.New()
	closed := time.Date(2026, 2, 19, 21, 0, 0, 0, time.UTC)
	if err := s.store.Upsert(ctx, ent.Name, id, map[string]any{
		"Наименование": "Ромашка",
		"Комментарий":  "важный клиент",
		"ДатаЗакрытия": closed,
		"Активен":      true,
		"Сумма":        "1234.50",
	}, ent); err != nil {
		t.Fatal(err)
	}

	// Форма отрисовала только Наименование — остальных ключей в POST нет.
	form := url.Values{"Наименование": {"Ромашка-2"}}
	r := reqWithChi("POST", "/ui/catalog/Клиент/"+id.String(), form,
		map[string]string{"entity": "Клиент", "id": id.String()})
	s.submitEdit(httptest.NewRecorder(), r)

	row, err := s.store.GetByID(ctx, ent.Name, id, ent)
	if err != nil {
		t.Fatal(err)
	}
	if got := row["Наименование"]; got != "Ромашка-2" {
		t.Errorf("присланное поле не применилось: %v", got)
	}
	if got := row["Комментарий"]; got != "важный клиент" {
		t.Errorf("строка затёрта: %v", got)
	}
	if row["ДатаЗакрытия"] == nil {
		t.Error("дата затёрта")
	}
	if row["Активен"] == nil {
		t.Error("bool затёрт")
	}
	if row["Сумма"] == nil {
		t.Error("число затёрто")
	}
}

// Присланный ПУСТОЙ ключ по-прежнему очищает поле: «применить» против
// «не передавалось» различаются наличием ключа, а не значением.
func TestSubmitEdit_ManagedForm_EmptyValueClears(t *testing.T) {
	ent := partialWriteEntity()
	ent.Forms = []*metadata.FormModule{managedObjectForm(
		fieldEl("ПолеНаим", "Объект.Наименование"),
		fieldEl("ПолеКомм", "Объект.Комментарий"),
	)}
	s, ctx := newSubmitTestServer(t, []*metadata.Entity{ent})

	id := uuid.New()
	if err := s.store.Upsert(ctx, ent.Name, id, map[string]any{
		"Наименование": "Ромашка", "Комментарий": "было",
	}, ent); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"Наименование": {"Ромашка"}, "Комментарий": {""}}
	r := reqWithChi("POST", "/ui/catalog/Клиент/"+id.String(), form,
		map[string]string{"entity": "Клиент", "id": id.String()})
	s.submitEdit(httptest.NewRecorder(), r)

	row, _ := s.store.GetByID(ctx, ent.Name, id, ent)
	if row["Комментарий"] != nil {
		t.Errorf("пустой присланный ключ должен очищать поле, получено %v", row["Комментарий"])
	}
}

// Снятие галочки должно работать: у не-ReadOnly Флажка отсутствие ключа —
// это «снято», а не «не передавалось». Guard против слишком широкого восстановления.
func TestSubmitEdit_ManagedForm_CheckboxUncheckClears(t *testing.T) {
	ent := partialWriteEntity()
	cb := &metadata.FormElement{Kind: metadata.FormElementType("Флажок"), Name: "ФлагАктивен", DataPath: "Объект.Активен"}
	ent.Forms = []*metadata.FormModule{managedObjectForm(fieldEl("ПолеНаим", "Объект.Наименование"), cb)}
	s, ctx := newSubmitTestServer(t, []*metadata.Entity{ent})

	id := uuid.New()
	if err := s.store.Upsert(ctx, ent.Name, id, map[string]any{
		"Наименование": "Ромашка", "Активен": true,
	}, ent); err != nil {
		t.Fatal(err)
	}
	// Чекбокс снят — браузер ключ не шлёт.
	form := url.Values{"Наименование": {"Ромашка"}}
	r := reqWithChi("POST", "/ui/catalog/Клиент/"+id.String(), form,
		map[string]string{"entity": "Клиент", "id": id.String()})
	s.submitEdit(httptest.NewRecorder(), r)

	row, _ := s.store.GetByID(ctx, ent.Name, id, ent)
	if isTruthyStored(row["Активен"]) {
		t.Errorf("снятие галочки не сработало: Активен=%v", row["Активен"])
	}
}

// А вот ReadOnly-Флажок пользователь снять не мог (контрол disabled) —
// его значение обязано сохраниться.
func TestSubmitEdit_ManagedForm_ReadOnlyCheckboxPreserved(t *testing.T) {
	ent := partialWriteEntity()
	cb := &metadata.FormElement{Kind: metadata.FormElementType("Флажок"), Name: "ФлагАктивен",
		DataPath: "Объект.Активен", ReadOnly: true}
	ent.Forms = []*metadata.FormModule{managedObjectForm(fieldEl("ПолеНаим", "Объект.Наименование"), cb)}
	s, ctx := newSubmitTestServer(t, []*metadata.Entity{ent})

	id := uuid.New()
	if err := s.store.Upsert(ctx, ent.Name, id, map[string]any{
		"Наименование": "Ромашка", "Активен": true,
	}, ent); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"Наименование": {"Ромашка"}}
	r := reqWithChi("POST", "/ui/catalog/Клиент/"+id.String(), form,
		map[string]string{"entity": "Клиент", "id": id.String()})
	s.submitEdit(httptest.NewRecorder(), r)

	row, _ := s.store.GetByID(ctx, ent.Name, id, ent)
	if !isTruthyStored(row["Активен"]) {
		t.Errorf("значение readonly-флажка потеряно: %v", row["Активен"])
	}
}

// Сущность БЕЗ управляемой формы работает по прежнему контракту: авто-форма
// рендерит все поля, а внешний частичный POST по-прежнему очищает опущенное.
func TestSubmitEdit_AutogenEntity_LegacyContractKept(t *testing.T) {
	ent := partialWriteEntity()
	ent.Forms = nil // авто-форма
	s, ctx := newSubmitTestServer(t, []*metadata.Entity{ent})

	id := uuid.New()
	if err := s.store.Upsert(ctx, ent.Name, id, map[string]any{
		"Наименование": "Ромашка", "Комментарий": "было",
	}, ent); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"Наименование": {"Ромашка"}}
	r := reqWithChi("POST", "/ui/catalog/Клиент/"+id.String(), form,
		map[string]string{"entity": "Клиент", "id": id.String()})
	s.submitEdit(httptest.NewRecorder(), r)

	row, _ := s.store.GetByID(ctx, ent.Name, id, ent)
	if row["Комментарий"] != nil {
		t.Errorf("контракт авто-формы изменился: Комментарий=%v", row["Комментарий"])
	}
}

// Иерархический справочник не должен улетать в корень и терять признак группы:
// managed-форма не рендерит parent_id/is_folder, а Upsert пишет их всегда.
func TestSubmitEdit_ManagedForm_HierarchyPreserved(t *testing.T) {
	ent := &metadata.Entity{
		Name: "Папка", Kind: metadata.KindCatalog, Hierarchical: true,
		Fields: []metadata.Field{{Name: "Наименование", Type: metadata.FieldTypeString}},
	}
	ent.Forms = []*metadata.FormModule{managedObjectForm(fieldEl("ПолеНаим", "Объект.Наименование"))}
	s, ctx := newSubmitTestServer(t, []*metadata.Entity{ent})

	parent := uuid.New()
	if err := s.store.Upsert(ctx, ent.Name, parent, map[string]any{
		"Наименование": "Родитель", "is_folder": true,
	}, ent); err != nil {
		t.Fatal(err)
	}
	child := uuid.New()
	if err := s.store.Upsert(ctx, ent.Name, child, map[string]any{
		"Наименование": "Группа", "is_folder": true, "parent_id": parent.String(),
	}, ent); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"Наименование": {"Группа-2"}}
	r := reqWithChi("POST", "/ui/catalog/Папка/"+child.String(), form,
		map[string]string{"entity": "Папка", "id": child.String()})
	s.submitEdit(httptest.NewRecorder(), r)

	row, _ := s.store.GetByID(ctx, ent.Name, child, ent)
	if !isTruthyStored(row["is_folder"]) {
		t.Errorf("признак группы снят: is_folder=%v", row["is_folder"])
	}
	if row["parent_id"] == nil {
		t.Error("элемент улетел в корень: parent_id=nil")
	}
}

// isTruthyStored — значение из БД может прийти как bool или как int64 (SQLite).
func isTruthyStored(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	}
	return false
}

func TestCheckboxOmittedFields_Unit(t *testing.T) {
	ent := &metadata.Entity{
		Name: "Клиент", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Активен", Type: metadata.FieldTypeBool},
			{Name: "Скрыт", Type: metadata.FieldTypeBool},
			{Name: "Комментарий", Type: metadata.FieldTypeString},
			{Name: "Цена", Type: metadata.FieldTypeNumber},
		},
	}
	cb := func(name, dp string, ro bool) *metadata.FormElement {
		return &metadata.FormElement{Kind: metadata.FormElementType("Флажок"), Name: name, DataPath: dp, ReadOnly: ro}
	}
	form := managedObjectForm(
		&metadata.FormElement{
			Kind: metadata.FormElementGroupBox, Name: "Группа",
			Children: []*metadata.FormElement{{
				Kind: metadata.FormElementType("Страница"), Name: "Стр",
				Children: []*metadata.FormElement{cb("Ф1", "Объект.Активен", false)},
			}},
		},
		cb("Ф2", "Объект.Скрыт", true),           // readonly — не в множестве
		cb("Ф3", "Объект.Комментарий", false),    // не bool — игнор
		cb("Ф4", "Объект.Товары.Активен", false), // путь ТЧ — не поле шапки
	)
	got := checkboxOmittedFields(form, ent)
	if !got["активен"] {
		t.Error("вложенный не-readonly Флажок не найден")
	}
	if got["скрыт"] {
		t.Error("readonly Флажок не должен попадать в множество")
	}
	if got["комментарий"] {
		t.Error("Флажок над не-bool полем не должен попадать в множество")
	}
	if len(got) != 1 {
		t.Errorf("лишние элементы: %v", got)
	}
}

func TestSubmittedFormKeys_Unit(t *testing.T) {
	body := url.Values{"Наименование": {"x"}, "Пустое": {""}}
	r := reqWithChi("POST", "/x?ИзQuery=1", body, nil)
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	keys := submittedFormKeys(r)
	if !formKeySubmitted(keys, "Наименование") {
		t.Error("ключ тела не распознан")
	}
	if !formKeySubmitted(keys, "Пустое") {
		t.Error("пустое значение — это ПРИСЛАННЫЙ ключ")
	}
	if !formKeySubmitted(keys, "наименование") {
		t.Error("проверка должна быть регистронезависимой")
	}
	if formKeySubmitted(keys, "ИзQuery") {
		t.Error("query-параметр не должен считаться присланным формой")
	}
}

func TestNormalizeRestoredValue_Bool(t *testing.T) {
	f := metadata.Field{Name: "Активен", Type: metadata.FieldTypeBool}
	for _, tc := range []struct {
		in   any
		want bool
	}{{int64(0), false}, {int64(1), true}, {true, true}, {false, false}, {"true", true}, {"false", false}} {
		if got := normalizeRestoredValue(f, tc.in); got != tc.want {
			t.Errorf("normalizeRestoredValue(%v (%T)) = %v, ожидалось %v", tc.in, tc.in, got, tc.want)
		}
	}
	// Не-bool поле не трогаем.
	s := metadata.Field{Name: "Комментарий", Type: metadata.FieldTypeString}
	if got := normalizeRestoredValue(s, int64(1)); got != int64(1) {
		t.Errorf("не-bool поле изменено: %v", got)
	}
}
