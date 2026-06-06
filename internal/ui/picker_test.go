package ui

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
)

// План 46, фаза 1: обработчик кнопки вызывает ПоказатьПодбор(Данные,Колонки,
// Конфиг). form-event должен вернуть pickerData с колонками, строками и
// конфигом, НЕ применяя ТЧ (диалог открывается на клиенте).
func TestPicker_ShowPickerReturnsPickerData(t *testing.T) {
	srv, ent := setupManagedEventsServer(t, `
Процедура ПодборНажатие()
	Колонки = Новый Массив;
	Колонки.Добавить(Новый Структура("Имя,Заголовок,Тип,Редактируемое", "Номенклатура", "Товар", "string", Ложь));
	Колонки.Добавить(Новый Структура("Имя,Заголовок,Тип,Редактируемое", "Количество", "Кол-во", "number", Истина));
	Данные = Новый Массив;
	Данные.Добавить(Новый Структура("Идентификатор,Номенклатура,Количество", "u-1", "Болт", 5));
	Данные.Добавить(Новый Структура("Идентификатор,Номенклатура,Количество", "u-2", "Гайка", 3));
	Конфиг = Новый Структура("Заголовок,ПолеПоиска,ПолеКоличества", "Подбор товаров", "Номенклатура", "Количество");
	ПоказатьПодбор(Данные, Колонки, Конфиг);
КонецПроцедуры
`, nil, []*metadata.FormElement{
		{
			Kind: metadata.FormElementButton,
			Name: "КнопкаПодбор",
			Handlers: map[metadata.FormEventType]string{
				metadata.FormEventOnClick: "ПодборНажатие",
			},
		},
	})

	body := url.Values{}
	body.Set("_element", "КнопкаПодбор")
	body.Set("_event", string(metadata.FormEventOnClick))

	rec := executeFormEvent(t, srv, ent, body)
	resp := decodeFormEventResponse(t, rec.Body.Bytes())

	if resp.PickerData == nil {
		t.Fatalf("ждали pickerData != nil; body=%s", rec.Body.String())
	}
	pd := resp.PickerData
	if len(pd.Columns) != 2 {
		t.Fatalf("ждали 2 колонки, получили %d (%+v)", len(pd.Columns), pd.Columns)
	}
	if pd.Columns[0].Name != "Номенклатура" || pd.Columns[0].Title != "Товар" {
		t.Errorf("колонка 0 = %+v", pd.Columns[0])
	}
	if !pd.Columns[1].Editable || pd.Columns[1].Type != "number" {
		t.Errorf("колонка «Количество» должна быть editable/number: %+v", pd.Columns[1])
	}
	if len(pd.Rows) != 2 {
		t.Fatalf("ждали 2 строки, получили %d", len(pd.Rows))
	}
	if pd.Rows[0].ID != "u-1" || pd.Rows[0].Data["Номенклатура"] != "Болт" {
		t.Errorf("строка 0 = %+v", pd.Rows[0])
	}
	if pd.Config.Title != "Подбор товаров" || pd.Config.SearchField != "Номенклатура" {
		t.Errorf("config = %+v", pd.Config)
	}
}

// План 46, фаза 2: _pick_result (JSON) разбирается в переменную ПодборРезультат,
// доступную обработчику события Выбор. Проверяем через Сообщить.
func TestPicker_PickResultParsedToVariable(t *testing.T) {
	srv, ent := setupManagedEventsServer(t, `
Процедура ПодборВыбор()
	Для Каждого Стр Из ПодборРезультат Цикл
		Сообщить(Стр.Номенклатура);
	КонецЦикла;
КонецПроцедуры
`, nil, []*metadata.FormElement{
		{
			Kind: metadata.FormElementButton,
			Name: "КнопкаПодбор",
			Handlers: map[metadata.FormEventType]string{
				metadata.FormEventOnChoice: "ПодборВыбор",
			},
		},
	})

	body := url.Values{}
	body.Set("_element", "КнопкаПодбор")
	body.Set("_event", string(metadata.FormEventOnChoice))
	body.Set("_pick_result", `[{"id":"u-1","Номенклатура":"Болт","Количество":"5"},{"id":"u-2","Номенклатура":"Гайка","Количество":"3"}]`)

	rec := executeFormEvent(t, srv, ent, body)
	resp := decodeFormEventResponse(t, rec.Body.Bytes())

	if !resp.OK {
		t.Fatalf("ok=false, error=%q", resp.Error)
	}
	if len(resp.Messages) != 2 || resp.Messages[0] != "Болт" || resp.Messages[1] != "Гайка" {
		t.Errorf("messages=%v, ждали [Болт Гайка]", resp.Messages)
	}
}

// parsePickResult: корректный JSON → Массив MapThis; пустой/битый → nil.
func TestParsePickResult(t *testing.T) {
	arr := parsePickResult(`[{"id":"a","Кол":"2"}]`)
	if arr == nil {
		t.Fatal("ждали непустой Массив")
	}
	items := arr.Iterate()
	if len(items) != 1 {
		t.Fatalf("ждали 1 элемент, получили %d", len(items))
	}
	mt, ok := items[0].(*interpreter.MapThis)
	if !ok {
		t.Fatalf("ждали *MapThis, получили %T", items[0])
	}
	if mt.Get("id") != "a" || mt.Get("кол") != "2" { // MapThis регистронезависим
		t.Errorf("значения не совпали: %+v", mt.M)
	}
	if parsePickResult("") != nil {
		t.Error("пустая строка должна давать nil")
	}
	if parsePickResult("{не json") != nil {
		t.Error("битый JSON должен давать nil")
	}
}

// selectedTPRows: _tp + _tp_selected → Массив выбранных строк ТЧ; пропускает
// невалидные/выходящие за границы индексы.
func TestSelectedTPRows(t *testing.T) {
	obj := &runtime.Object{
		ID:   uuid.New(),
		Type: "Реализация",
		TablePartRows: map[string][]map[string]any{
			"Товары": {
				{"Номенклатура": "Болт", "Количество": "1"},
				{"Номенклатура": "Гайка", "Количество": "2"},
				{"Номенклатура": "Шайба", "Количество": "3"},
			},
		},
	}
	form := url.Values{}
	form.Set("_tp", "Товары")
	form.Set("_tp_selected", "0,2,99") // 99 за границей — игнор
	req := httptest.NewRequest("POST", "/x", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}

	arr := selectedTPRows(req, obj)
	if arr == nil {
		t.Fatal("ждали непустой Массив выделенных строк")
	}
	items := arr.Iterate()
	if len(items) != 2 {
		t.Fatalf("ждали 2 строки (0 и 2), получили %d", len(items))
	}
	r0 := items[0].(*interpreter.MapThis)
	r1 := items[1].(*interpreter.MapThis)
	if r0.Get("Номенклатура") != "Болт" || r1.Get("Номенклатура") != "Шайба" {
		t.Errorf("выбраны не те строки: %v / %v", r0.M, r1.M)
	}

	// Без _tp_selected — nil.
	form2 := url.Values{}
	form2.Set("_tp", "Товары")
	req2 := httptest.NewRequest("POST", "/x", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.ParseForm()
	if selectedTPRows(req2, obj) != nil {
		t.Error("без _tp_selected ждали nil")
	}
}
