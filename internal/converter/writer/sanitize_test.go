package writer

import (
	"strings"
	"testing"
)

// Директивы препроцессора 1С (#Область, #Если…) не поддерживаются DSL —
// загрузка конфигурации падала на «expected Procedure or Function, got "#"»
// (issue #48 п.2). Конвертер обязан их вырезать, сохраняя содержимое блоков.
func TestSanitizeBSL(t *testing.T) {
	in := strings.Join([]string{
		"#Область Сервис",
		"Процедура Привет() Экспорт",
		"  #Если Сервер Тогда",
		"  а = 1;",
		"  #ИначеЕсли Клиент Тогда",
		"  а = 2;",
		"  #Иначе",
		"  а = 3;",
		"  #КонецЕсли",
		"КонецПроцедуры",
		"#КонецОбласти",
		"#Region English",
		"#EndRegion",
	}, "\n")
	got := sanitizeBSL(in)
	if strings.Contains(got, "#") {
		t.Fatalf("директивы не вырезаны:\n%s", got)
	}
	for _, want := range []string{"Процедура Привет() Экспорт", "а = 1;", "а = 2;", "а = 3;", "КонецПроцедуры"} {
		if !strings.Contains(got, want) {
			t.Fatalf("потеряно содержимое %q:\n%s", want, got)
		}
	}
}
