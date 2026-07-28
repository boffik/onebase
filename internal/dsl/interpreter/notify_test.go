package interpreter

import "testing"

// fakeNotifier структурно удовлетворяет интерфейсу издателя уведомлений.
type fakeNotifier struct {
	target, name string
	data         any
	calls        int
}

func (f *fakeNotifier) Publish(target, name string, data any) {
	f.target, f.name, f.data = target, name, data
	f.calls++
}

func TestNewNotifyFunctions_PublishesWithArgs(t *testing.T) {
	fn := &fakeNotifier{}
	pub, ok := NewNotifyFunctions(fn)["ОтправитьУведомление"].(BuiltinFunc)
	if !ok {
		t.Fatal("ОтправитьУведомление не зарегистрирована как BuiltinFunc")
	}
	if _, err := pub([]any{"ivan", "звонок.входящий", "+79990001122"}, "", 0); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if fn.calls != 1 || fn.target != "ivan" || fn.name != "звонок.входящий" || fn.data != "+79990001122" {
		t.Fatalf("издатель получил неверные аргументы: %+v", fn)
	}
}

func TestNewNotifyFunctions_NilNotifierIsNoop(t *testing.T) {
	pub := NewNotifyFunctions(nil)["ОтправитьУведомление"].(BuiltinFunc)
	if _, err := pub([]any{"ivan", "x"}, "", 0); err != nil {
		t.Fatalf("без подключённой шины функция должна быть no-op, получено: %v", err)
	}
}

func TestNewNotifyFunctions_ConvertsStructData(t *testing.T) {
	fn := &fakeNotifier{}
	pub := NewNotifyFunctions(fn)["ОтправитьУведомление"].(BuiltinFunc)
	pub([]any{"ivan", "ui.обновитьСписок", NewStructFromMap(map[string]any{"сущность": "Заявка"})}, "", 0)
	m, ok := fn.data.(map[string]any)
	if !ok {
		t.Fatalf("DSL-структуру нужно конвертировать в map (иначе на клиент придёт {}): %T", fn.data)
	}
	if m["сущность"] != "Заявка" {
		t.Fatalf("поле не донесено на клиент: %+v", m)
	}
}

func TestShowUserNotification_PublishesUINotice(t *testing.T) {
	fn := &fakeNotifier{}
	show, ok := NewNotifyFunctions(fn)["ПоказатьОповещениеПользователя"].(BuiltinFunc)
	if !ok {
		t.Fatal("ПоказатьОповещениеПользователя не зарегистрирована")
	}
	link := NewStructFromMap(map[string]any{"вид": "document", "сущность": "Задача", "id": "u-1"})
	if _, err := show([]any{"ivan", "Новая задача", "Согласуйте", link, "важное"}, "", 0); err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if fn.calls != 1 || fn.target != "ivan" || fn.name != "ui.оповещение" {
		t.Fatalf("неверная публикация: %+v", fn)
	}
	m, ok := fn.data.(map[string]any)
	if !ok {
		t.Fatalf("данные не map[string]any: %T", fn.data)
	}
	if m["заголовок"] != "Новая задача" || m["текст"] != "Согласуйте" || m["важность"] != "важное" {
		t.Fatalf("payload неверный: %+v", m)
	}
	ref, ok := m["ссылка"].(map[string]any)
	if !ok || ref["сущность"] != "Задача" || ref["вид"] != "document" {
		t.Fatalf("ссылка не сериализована в JSON-map: %+v (%T)", m["ссылка"], m["ссылка"])
	}
	// Важность по умолчанию — «обычное»; алиас ShowUserNotification работает.
	fn2 := &fakeNotifier{}
	NewNotifyFunctions(fn2)["ShowUserNotification"].(BuiltinFunc)([]any{"ivan", "T", "X"}, "", 0)
	if m2 := fn2.data.(map[string]any); m2["важность"] != "обычное" {
		t.Fatalf("важность по умолчанию должна быть обычное: %+v", m2)
	}
}

func TestNewNotifyFunctions_RequiresTargetAndEvent(t *testing.T) {
	fn := &fakeNotifier{}
	pub := NewNotifyFunctions(fn)["ОтправитьУведомление"].(BuiltinFunc)
	if _, err := pub([]any{"ivan"}, "", 0); err == nil {
		t.Fatal("ожидалась ошибка при недостатке аргументов (нет события)")
	}
	if fn.calls != 0 {
		t.Fatal("при ошибке аргументов Publish вызываться не должен")
	}
}
