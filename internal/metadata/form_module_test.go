package metadata

import "testing"

// события табличных частей определены как well-known
// константы, чтобы YAML-конфиг мог их декларировать единым именованием.
func TestFormEventTypes_RowEvents(t *testing.T) {
	cases := []struct {
		event FormEventType
		want  string
	}{
		{FormEventOnRowAdded, "ПриДобавленииСтроки"},
		{FormEventOnRowChanged, "ПриИзмененииСтроки"},
		{FormEventOnRowDeleted, "ПриУдаленииСтроки"},
		{FormEventOnRowActivated, "ПриАктивизацииСтроки"},
	}
	for _, c := range cases {
		if string(c.event) != c.want {
			t.Errorf("event %v = %q, want %q", c.event, c.event, c.want)
		}
	}
}

// FormModule.GetEventHandler должен находить обработчики на табличной
// части по новым event-типам.
func TestFormModule_GetEventHandler_RowAdded(t *testing.T) {
	form := &FormModule{
		Elements: []*FormElement{
			{
				ID:   "товары",
				Name: "Товары",
				Kind: FormElementTablePart,
				Handlers: map[FormEventType]string{
					FormEventOnRowAdded: "ПодставитьЦенуПриДобавлении",
				},
			},
		},
	}
	handler, ok := form.GetEventHandler("Товары", FormEventOnRowAdded)
	if !ok {
		t.Fatal("обработчик ПриДобавленииСтроки не найден")
	}
	if handler != "ПодставитьЦенуПриДобавлении" {
		t.Errorf("неверный handler: %q", handler)
	}
}
