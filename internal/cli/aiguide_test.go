package cli

import (
	"strings"
	"testing"
)

func TestGenerateAIGuide_HasSignaturesAndSections(t *testing.T) {
	g := generateAIGuide("")
	for _, want := range []string{
		"## Язык DSL",
		"### Методы объектов",
		"### Язык запросов",
		"СтрЗаменить(",     // сигнатура функции
		"Запрос.Выполнить", // метод объекта
		// секция-протокол «Проверка результата» (план 91)
		"## Проверка результата",
		"procrun",
		"UI не проверен headless",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("в guide нет ожидаемого фрагмента: %q", want)
		}
	}
	if strings.Contains(g, "Сигнатуры смотрите в примерах") {
		t.Error("guide всё ещё содержит устаревший дисклеймер о сигнатурах")
	}
	// По умолчанию (без проекта) руководство портируемо — без блока «Окружение».
	if strings.Contains(g, "## Окружение") {
		t.Error("guide без проекта не должен содержать discovered-блок «Окружение»")
	}
}

// TestEnvironmentNote — discovered-блок «Окружение» (план 91): пуст без проекта
// (портируемость по умолчанию), с путём и ОС-заметкой при заданном каталоге.
func TestEnvironmentNote(t *testing.T) {
	if got := environmentNote(""); got != "" {
		t.Errorf("environmentNote(\"\") должно быть пустым для портируемости, получено: %q", got)
	}
	note := environmentNote(".")
	for _, want := range []string{"## Окружение", "- Проект:", "- ОС:"} {
		if !strings.Contains(note, want) {
			t.Errorf("environmentNote(dir) не содержит %q", want)
		}
	}
	// Блок встраивается в руководство при генерации с проектом.
	if !strings.Contains(generateAIGuide(note), "## Окружение") {
		t.Error("generateAIGuide(envBlock) должно включать блок «Окружение»")
	}
}
