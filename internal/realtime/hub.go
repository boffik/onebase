// Package realtime — внутрипроцессная шина уведомлений «сервер → браузер».
// Hub маршрутизирует события подписчикам (открытым SSE-соединениям) по адресу:
// логин пользователя, "роль:<Имя>" или "*" (всем онлайн). Область действия —
// один процесс onebase (как БлокировкаДанных); горизонтальное масштабирование
// потребует внешнего брокера (Redis/NATS) — отдельный план.
package realtime

import (
	"strconv"
	"strings"
	"sync"
)

// rolePrefix — адрес вида "роль:Оператор" доставляет событие всем подписчикам с
// этой ролью.
const rolePrefix = "роль:"

// Event — одно уведомление: имя события и произвольные данные (сериализуются в
// JSON на стороне SSE-эндпоинта).
type Event struct {
	Name string
	Data any
}

// subscriberBuffer — ёмкость канала одного подписчика. При переполнении кадры
// дропаются (см. Publish), издатель не блокируется медленным клиентом.
const subscriberBuffer = 32

type subscriber struct {
	id    string
	login string
	roles []string
	ch    chan Event
}

// Hub — потокобезопасный реестр подписчиков.
type Hub struct {
	mu   sync.Mutex
	subs map[string]*subscriber
	seq  int
}

// NewHub создаёт пустую шину.
func NewHub() *Hub {
	return &Hub{subs: make(map[string]*subscriber)}
}

// Subscribe регистрирует подписчика и возвращает его id, канал событий и функцию
// отписки. cancel закрывает канал и удаляет подписчика; вызывать при завершении
// SSE-соединения.
func (h *Hub) Subscribe(userID, login string, roles []string) (id string, ch <-chan Event, cancel func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	id = "s" + strconv.Itoa(h.seq)
	s := &subscriber{id: id, login: login, roles: roles, ch: make(chan Event, subscriberBuffer)}
	h.subs[id] = s
	return id, s.ch, func() { h.unsubscribe(id) }
}

func (h *Hub) unsubscribe(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.subs[id]; ok {
		delete(h.subs, id)
		close(s.ch)
	}
}

// Publish доставляет событие всем подписчикам, чей адрес совпал с target.
// Отправка неблокирующая: если буфер подписчика полон, кадр для него дропается.
func (h *Hub) Publish(target string, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.subs {
		if matches(s, target) {
			select {
			case s.ch <- ev:
			default: // буфер полон — дропаем кадр, не блокируем издателя
			}
		}
	}
}

// matches решает, адресовано ли событие подписчику.
func matches(s *subscriber, target string) bool {
	if target == "*" {
		return true
	}
	if role := strings.TrimPrefix(target, rolePrefix); role != target {
		for _, r := range s.roles {
			if r == role {
				return true
			}
		}
		return false
	}
	return s.login == target
}
