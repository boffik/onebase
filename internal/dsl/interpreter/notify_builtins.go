package interpreter

import "fmt"

// Notifier публикует уведомление в real-time-шину «сервер → браузер» (план 74).
// Интерфейс объявлен здесь, чтобы пакет interpreter не зависел от
// internal/realtime; конкретную реализацию (адаптер над *realtime.Hub)
// инжектирует слой UI/конфигурации через dslvars.
type Notifier interface {
	// Publish доставляет событие по адресу target (логин | "роль:<Имя>" | "*").
	Publish(target, name string, data any)
}

// NewNotifyFunctions возвращает DSL-функции публикации уведомлений
// (ОтправитьУведомление / PublishNotification). Если n == nil — функции
// остаются тихим no-op (фоновые задания, тесты, не подключённая шина),
// поэтому конфигурация с вызовом не падает там, где push недоступен.
func NewNotifyFunctions(n Notifier) map[string]any {
	publish := BuiltinFunc(func(args []any, _ string, _ int) (any, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("ОтправитьУведомление: ожидаются аргументы (Кому, Событие[, Данные])")
		}
		if n == nil {
			return nil, nil
		}
		var data any
		if len(args) >= 3 {
			data = args[2]
		}
		n.Publish(notifyArgString(args[0]), notifyArgString(args[1]), data)
		return nil, nil
	})
	return map[string]any{
		"ОтправитьУведомление": publish,
		"PublishNotification":  publish,
	}
}

// notifyArgString приводит адрес/имя события к строке (nil → "").
func notifyArgString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
