package interpreter

import "testing"

// НайтиПоРеквизиту ищет по произвольному полю через FindCatalogByField.
func TestCatalogProxy_FindByAttribute(t *testing.T) {
	root, db, _ := newCatalogsTestEnv()
	db.byField["ТипЦен/Код"] = map[string]struct{ ID, Display string }{
		"СЦ-001": {ID: "33333333-3333-3333-3333-333333333333", Display: "Спеццена"},
	}
	cp := root.Get("ТипЦен").(*CatalogProxy)

	v := cp.CallMethod("найтипореквизиту", []any{"Код", "СЦ-001"})
	ref, ok := v.(*Ref)
	if !ok {
		t.Fatalf("ожидался *Ref, получили %T", v)
	}
	if ref.UUID != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("неверный UUID: %s", ref.UUID)
	}
	if got := cp.CallMethod("найтипореквизиту", []any{"Код", "НЕТ"}); got != nil {
		t.Errorf("не найдено → nil, got %v", got)
	}
}

// ЭтоНовый — Истина у только что созданного объекта, Ложь после Записать.
// Прочитать — перечитывает поля из БД.
func TestCatalogWriter_IsNewAndRead(t *testing.T) {
	root, db, _ := newCatalogsTestEnv()
	cp := root.Get("ТипЦен").(*CatalogProxy)
	w := cp.CallMethod("создать", nil).(*CatalogRecordWriter)

	if w.CallMethod("этоновый", nil) != true {
		t.Error("новый объект: ЭтоНовый должно быть Истина")
	}
	w.Set("Наименование", "X")
	w.CallMethod("записать", nil)
	if w.CallMethod("этоновый", nil) != false {
		t.Error("после Записать: ЭтоНовый должно быть Ложь")
	}

	db.stored = map[string]map[string]any{
		"ТипЦен/99999999-9999-9999-9999-999999999999": {"Наименование": "ИзБазы"},
	}
	w.CallMethod("прочитать", nil)
	if got := w.Get("Наименование"); got != "ИзБазы" {
		t.Errorf("после Прочитать ожидалось ИзБазы, got %v", got)
	}
}

func TestFillPropertyValues_All(t *testing.T) {
	src := newStruct([]any{"Имя, Возраст, Город", "Иван", float64(30), "Москва"})
	dst := newStruct([]any{"Имя, Возраст, Город"})
	if _, err := fillPropertyValuesFn([]any{dst, src}, "", 0); err != nil {
		t.Fatal(err)
	}
	if dst.Get("Имя") != "Иван" || dst.Get("Возраст") != float64(30) || dst.Get("Город") != "Москва" {
		t.Errorf("копирование всех свойств: dst=%v", dst.vals)
	}
}

func TestFillPropertyValues_IncludeList(t *testing.T) {
	src := newStruct([]any{"Имя, Возраст, Город", "Иван", float64(30), "Москва"})
	dst := newStruct([]any{"Имя, Возраст, Город"})
	if _, err := fillPropertyValuesFn([]any{dst, src, "Имя, Город"}, "", 0); err != nil {
		t.Fatal(err)
	}
	if dst.Get("Имя") != "Иван" || dst.Get("Город") != "Москва" {
		t.Errorf("включённые свойства не скопированы: %v", dst.vals)
	}
	if dst.Get("Возраст") != nil {
		t.Errorf("Возраст не должен копироваться (нет в списке): %v", dst.Get("Возраст"))
	}
}

func TestFillPropertyValues_ExcludeList(t *testing.T) {
	src := newStruct([]any{"Имя, Возраст, Город", "Иван", float64(30), "Москва"})
	dst := newStruct([]any{"Имя, Возраст, Город"})
	// 3-й аргумент пустой → «все свойства», 4-й — исключаемые.
	if _, err := fillPropertyValuesFn([]any{dst, src, "", "Город"}, "", 0); err != nil {
		t.Fatal(err)
	}
	if dst.Get("Имя") != "Иван" || dst.Get("Возраст") != float64(30) {
		t.Errorf("неисключённые свойства не скопированы: %v", dst.vals)
	}
	if dst.Get("Город") != nil {
		t.Errorf("Город должен быть исключён: %v", dst.Get("Город"))
	}
}
