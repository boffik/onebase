package webhook

import (
	"encoding/json"
	"testing"
)

// #12: базовые поля события (user/entity/id) должны экранироваться в JSON-теле.
// Логин со спецсимволами раньше ломал JSON или инъецировал поля в payload.
func TestRenderBody_EscapesBaseFields(t *testing.T) {
	e := Event{
		ID:     "11111111-1111-1111-1111-111111111111",
		Entity: "Заказ",
		User:   "a\"b\nc", // кавычка + перевод строки в логине
		Record: map[string]any{"Комментарий": "ok"},
	}
	body, err := renderBody(`{"user":"{{user}}","entity":"{{entity}}","id":"{{id}}","c":"{{Комментарий}}"}`, e)
	if err != nil {
		t.Fatalf("renderBody: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("тело должно оставаться валидным JSON при спецсимволах в логине: %v\nbody=%s", err, body)
	}
	if m["user"] != "a\"b\nc" {
		t.Errorf("user исказился после экранирования: %q", m["user"])
	}
	if m["entity"] != "Заказ" {
		t.Errorf("entity исказился: %q", m["entity"])
	}
}
