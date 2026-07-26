package ui

import (
	"context"
	"testing"
	"time"

	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/realtime"
	"github.com/ivantit66/onebase/internal/runtime"
)

func orderRegistry(notify bool) *runtime.Registry {
	order := &metadata.Entity{
		Name: "Заказ", Kind: metadata.KindDocument, NotifyChanges: notify,
		Fields: []metadata.Field{{Name: "Ответственный", Type: "string"}},
	}
	reg := runtime.NewRegistry()
	reg.Load(runtime.LoadOptions{Entities: []*metadata.Entity{order}})
	return reg
}

// operatorUser — пользователь с ролью, видящей только «свои» заказы
// (Ответственный = свой логин). Проверяет ядро адресации живого списка.
func operatorUser(login string) *auth.User {
	role := &auth.Role{Name: "Оператор", Permissions: auth.Permission{
		Documents: map[string][]string{"Заказ": {"read"}},
		RowAccess: auth.RowAccess{Documents: map[string]auth.RowPolicies{
			"Заказ": {"read": auth.RowPolicy{Field: "Ответственный", Op: "eq", Value: auth.RowValue{User: "login"}}},
		}},
	}}
	return &auth.User{ID: "id-" + login, Login: login, Roles: []*auth.Role{role}}
}

func TestChangePublisher_CanSee_RLS(t *testing.T) {
	reg := orderRegistry(true)
	p := &changePublisher{s: &Server{reg: reg}}
	entity := reg.GetEntity("Заказ")
	ctx := context.Background()
	ivan := operatorUser("ivan")

	if !p.canSee(ctx, ivan, entity, map[string]any{"ответственный": "ivan"}) {
		t.Fatal("ivan должен видеть свой заказ")
	}
	if p.canSee(ctx, ivan, entity, map[string]any{"ответственный": "petr"}) {
		t.Fatal("ivan НЕ должен видеть чужой заказ (утечка адресации)")
	}
	if p.canSee(ctx, ivan, entity, nil) {
		t.Fatal("nil-образ (нет строки) → недоступен")
	}
	// Админ — без ограничений.
	admin := &auth.User{ID: "a", Login: "boss", IsAdmin: true}
	if !p.canSee(ctx, admin, entity, map[string]any{"ответственный": "petr"}) {
		t.Fatal("админ видит любую строку")
	}
	// Пользователь без прав на Заказ — не видит.
	noperm := &auth.User{ID: "n", Login: "guest", Roles: []*auth.Role{{Name: "Пусто"}}}
	if p.canSee(ctx, noperm, entity, map[string]any{"ответственный": "guest"}) {
		t.Fatal("без права read строка недоступна")
	}
}

func recvUI(t *testing.T, ch <-chan realtime.Event) (realtime.Event, bool) {
	t.Helper()
	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(time.Second):
		return realtime.Event{}, false
	}
}

func emptyUI(t *testing.T, ch <-chan realtime.Event) bool {
	t.Helper()
	select {
	case ev := <-ch:
		t.Logf("неожиданное событие: %+v", ev)
		return false
	case <-time.After(150 * time.Millisecond):
		return true
	}
}

func TestChangePublisher_LegacyBroadcast(t *testing.T) {
	hub := realtime.NewHub()
	defer hub.Close()
	s := &Server{hub: hub, reg: orderRegistry(true), authRepo: nil} // нет auth → легаси
	_, ch, cancel := hub.Subscribe("u1", "ivan", nil)
	defer cancel()

	s.newChangePublisher().PublishChange(context.Background(), "Заказ", "записан", nil, map[string]any{"ответственный": "ivan"})

	ev, ok := recvUI(t, ch)
	if !ok || ev.Name != "данные.заказ" {
		t.Fatalf("в легаси-режиме ожидался broadcast данные.заказ: ok=%v ev=%+v", ok, ev)
	}
}

func TestChangePublisher_NotifyChangesGate(t *testing.T) {
	hub := realtime.NewHub()
	defer hub.Close()
	s := &Server{hub: hub, reg: orderRegistry(false), authRepo: nil} // notify_changes выключен
	_, ch, cancel := hub.Subscribe("u1", "ivan", nil)
	defer cancel()

	s.newChangePublisher().PublishChange(context.Background(), "Заказ", "записан", nil, map[string]any{"ответственный": "ivan"})

	if !emptyUI(t, ch) {
		t.Fatal("без notify_changes событие публиковаться не должно")
	}
}
