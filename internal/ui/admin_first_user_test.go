package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestAdminUserForm_FirstUserWarnsAndChecksAdmin(t *testing.T) {
	var buf bytes.Buffer
	err := adminTmpl.ExecuteTemplate(&buf, "admin-user-form", map[string]any{
		"FirstUser":    true,
		"AdminChecked": true,
	})
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Первый пользователь должен быть администратором") {
		t.Error("форма первого пользователя должна содержать предупреждение")
	}
	if !strings.Contains(html, `name="is_admin" value="1" checked`) {
		t.Error("чекбокс Администратор должен быть отмечен по умолчанию")
	}
}

func TestAdminUsersList_EmptyHintMentionsAdmin(t *testing.T) {
	var buf bytes.Buffer
	err := adminTmpl.ExecuteTemplate(&buf, "admin-users", map[string]any{"Users": nil})
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	if !strings.Contains(buf.String(), "Первый пользователь должен быть администратором") {
		t.Error("пустой список должен требовать первого администратора")
	}
}
