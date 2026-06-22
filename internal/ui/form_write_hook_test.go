package ui

// Направление 3 (Фаза B): серверные события записи формы
// ПередЗаписью/ПриЗаписи/ПослеЗаписи. Раньше они объявлялись, но молча не
// вызывались в save-пути. Здесь проверяем: ПередЗаписью может отменить запись
// (исключение) и мутировать реквизиты так, что мутация доходит до Save
// (сведение регистра ключей), а ПослеЗаписи исполняется после записи.

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/runtime"
)

// ПередЗаписью мутирует реквизит через Объект.Поле = … (Object.Set пишет ключ в
// нижнем регистре). Для существующего объекта реквизиты приходят в оригинальном
// регистре — без сведения у поля было бы два ключа и Save прочитал бы старое.
// runPreSaveFormHooks обязан свести их: победить должна мутация хука.
func TestRunPreSaveFormHooks_MutationReachesSave(t *testing.T) {
	srv, ent := setupManagedEventsServer(t, `
Процедура ПередЗаписьюФормы()
	Объект.Наименование = "НОВОЕ";
КонецПроцедуры
`, map[metadata.FormEventType]string{
		metadata.FormEventBeforeWrite: "ПередЗаписьюФормы",
	}, []*metadata.FormElement{
		{Kind: metadata.FormElementField, Name: "Наименование", DataPath: "Объект.Наименование"},
	})

	// Существующий объект: ключи в оригинальном регистре (как из formToFields).
	obj := &runtime.Object{
		ID:            uuid.New(),
		Type:          ent.Name,
		Kind:          ent.Kind,
		Fields:        map[string]any{"Наименование": "СТАРОЕ"},
		TablePartRows: map[string][]map[string]any{},
	}

	var msgs []string
	if err := srv.runPreSaveFormHooks(context.Background(), ent, obj, &msgs); err != nil {
		t.Fatalf("runPreSaveFormHooks вернул ошибку: %v", err)
	}

	if got := obj.Fields["Наименование"]; got != "НОВОЕ" {
		t.Errorf("Fields[Наименование] = %v, ждали НОВОЕ (мутация хука не дошла)", got)
	}
	if _, dup := obj.Fields["наименование"]; dup {
		t.Errorf("остался дубль-ключ в нижнем регистре: %v", obj.Fields)
	}
	if len(obj.Fields) != 1 {
		t.Errorf("ожидался один ключ поля, получено %v", obj.Fields)
	}
}

// ПередЗаписью с ВызватьИсключение отменяет запись: runPreSaveFormHooks
// возвращает ошибку, а вызывающий код перерисует форму и не вызовет Save.
func TestRunPreSaveFormHooks_Abort(t *testing.T) {
	srv, ent := setupManagedEventsServer(t, `
Процедура ПередЗаписьюФормы()
	ВызватьИсключение("Укажите заголовок");
КонецПроцедуры
`, map[metadata.FormEventType]string{
		metadata.FormEventBeforeWrite: "ПередЗаписьюФормы",
	}, []*metadata.FormElement{
		{Kind: metadata.FormElementField, Name: "Наименование", DataPath: "Объект.Наименование"},
	})

	obj := &runtime.Object{ID: uuid.New(), Type: ent.Name, Kind: ent.Kind,
		Fields: map[string]any{"Наименование": ""}, TablePartRows: map[string][]map[string]any{}}

	var msgs []string
	err := srv.runPreSaveFormHooks(context.Background(), ent, obj, &msgs)
	if err == nil {
		t.Fatal("ожидалась ошибка (ПередЗаписью бросил исключение), получили nil")
	}
	if !strings.Contains(err.Error(), "заголовок") {
		t.Errorf("ошибка = %q, ждали текст исключения", err.Error())
	}
}

// Если форма не объявляет события записи — runPreSaveFormHooks no-op: Объект не
// трогается (поведение save как раньше, без накладных расходов).
func TestRunPreSaveFormHooks_NoHookNoop(t *testing.T) {
	srv, ent := setupManagedEventsServer(t, ``, nil,
		[]*metadata.FormElement{
			{Kind: metadata.FormElementButton, Name: "КнопкаПусто"},
		})

	obj := &runtime.Object{ID: uuid.New(), Type: ent.Name, Kind: ent.Kind,
		Fields: map[string]any{"Наименование": "X"}, TablePartRows: map[string][]map[string]any{}}

	var msgs []string
	if err := srv.runPreSaveFormHooks(context.Background(), ent, obj, &msgs); err != nil {
		t.Fatalf("no-op ждали nil, получили %v", err)
	}
	if obj.Fields["Наименование"] != "X" || len(obj.Fields) != 1 {
		t.Errorf("Объект изменился без хуков: %v", obj.Fields)
	}
}

// ПослеЗаписи исполняется после успешной записи с перезагруженным из БД
// Объектом (с актуальными значениями реквизитов).
func TestRunAfterWriteFormHook(t *testing.T) {
	srv, ent := setupManagedEventsServer(t, `
Процедура ПослеЗаписиФормы()
	Сообщить("записано: " + Объект.Наименование);
КонецПроцедуры
`, map[metadata.FormEventType]string{
		metadata.FormEventAfterWrite: "ПослеЗаписиФормы",
	}, []*metadata.FormElement{
		{Kind: metadata.FormElementField, Name: "Наименование", DataPath: "Объект.Наименование"},
	})

	id := insertContragent(t, srv, ent, "КОНТРАГЕНТ-1")

	var msgs []string
	srv.runAfterWriteFormHook(context.Background(), ent, id, &msgs)

	if len(msgs) != 1 || !strings.Contains(msgs[0], "КОНТРАГЕНТ-1") {
		t.Errorf("messages = %v, ждали сообщение с наименованием записанного объекта", msgs)
	}
}
