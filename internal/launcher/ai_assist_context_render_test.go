package launcher

import (
	"strings"
	"testing"
)

// Баг: чекбокс «добавить текущий код из активного редактора» отправлял модели
// содержимое ПЕРВОГО созданного Monaco-редактора (порядок monaco.editor.
// getEditors()), а не активного — редакторы накапливаются при переключении
// панелей и не уничтожаются. activeCode() должен использовать механизм
// активного редактора (_lastFocusedEditorId, как у синтакс-помощника) и код
// видимой панели (.cfg-panel.active), а не глобальный перебор редакторов.
func TestConfigurator_AIAssistActiveCode(t *testing.T) {
	html := renderCfgFoot(t)
	i := strings.Index(html, "function activeCode()")
	if i < 0 {
		t.Fatal("в cfg-foot нет function activeCode()")
	}
	j := strings.Index(html[i:], "send.addEventListener")
	if j < 0 {
		t.Fatal("после activeCode() нет send.addEventListener — структура скрипта изменилась, поправь срез в тесте")
	}
	fn := html[i : i+j]
	for _, sub := range []string{"_lastFocusedEditorId", ".cfg-panel.active"} {
		if !strings.Contains(fn, sub) {
			t.Errorf("activeCode() не использует %q — контекст ИИ может уйти не из активного редактора", sub)
		}
	}
	if strings.Contains(fn, "getEditors") {
		t.Error("activeCode() всё ещё перебирает monaco.editor.getEditors() — вернётся первый созданный редактор, а не активный")
	}
}

// Если контекст с галочкой не нашёлся (нет видимой панели с кодом), пользователь
// должен видеть предупреждение, а не гадать, почему модель «не видит» код.
func TestConfigurator_AIAssistNoContextWarning(t *testing.T) {
	html := renderCfgFoot(t)
	if !strings.Contains(html, "запрос без контекста") {
		t.Error("в скрипте ИИ-панели нет предупреждения об отправке запроса без контекста кода")
	}
}
