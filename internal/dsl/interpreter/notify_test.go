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
