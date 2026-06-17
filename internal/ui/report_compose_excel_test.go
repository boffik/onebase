package ui

import (
	"testing"

	"github.com/ivantit66/onebase/internal/report"
	"github.com/ivantit66/onebase/internal/report/compose"
)

// TestComposedRows проверяет порядок строк и содержимое ключевых ячеек.
//
// Структура: 2 группы (Иванов, Петров), подытоги, общий итог.
// Ожидаемый порядок (при Subtotals:true, Grand:true):
//
//	[0] заголовки     ("М / Отдел", "Сумма, ₽")
//	[1] группа Иванов (уровень 0)
//	[2] дочерняя      Отдел A (уровень 1)
//	[3] подытог       Отдел A
//	[4] подытог       Иванов
//	[5] группа Петров (уровень 0)
//	[6] дочерняя      Отдел B (уровень 1)
//	[7] подытог       Отдел B
//	[8] подытог       Петров
//	[9] ВСЕГО
func TestComposedRows(t *testing.T) {
	rows := []compose.Row{
		{"М": "Иванов", "Отдел": "Отдел A", "Сумма": "100"},
		{"М": "Петров", "Отдел": "Отдел B", "Сумма": "50"},
	}
	spec := report.Composition{
		Groupings: []string{"М", "Отдел"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum", Title: "Сумма, ₽"}},
		Totals:    report.Totals{Grand: true, Subtotals: true},
	}
	res, err := compose.Compose(rows, spec, nil)
	if err != nil {
		t.Fatalf("compose error: %v", err)
	}

	headers, xlsRows := composedRows(res, &spec)

	// Проверяем заголовки
	if len(headers) != 2 {
		t.Fatalf("ожидали 2 заголовка, получили %d: %v", len(headers), headers)
	}
	if headers[0] != "М / Отдел" {
		t.Errorf("headers[0] = %q, ожидали %q", headers[0], "М / Отдел")
	}
	if headers[1] != "Сумма, ₽" {
		t.Errorf("headers[1] = %q, ожидали %q", headers[1], "Сумма, ₽")
	}

	// Ожидаем 9 строк данных:
	// 2 группы × (1 группа + 1 дочерняя + 1 подытог дочерней + 1 подытог) + 1 ВСЕГО
	wantRows := 9
	if len(xlsRows) != wantRows {
		t.Fatalf("ожидали %d строк, получили %d", wantRows, len(xlsRows))
	}

	// Строка 0: группа Иванов (уровень 0) — первая колонка с нулевым отступом
	row0 := xlsRows[0]
	if len(row0) != 2 {
		t.Fatalf("строка 0: ожидали 2 ячейки, получили %d", len(row0))
	}
	grpLabel0, _ := row0[0].(string)
	if grpLabel0 != "Иванов" {
		t.Errorf("строка 0 [0] = %q, ожидали %q", grpLabel0, "Иванов")
	}
	// Значение показателя должно быть числом (float64)
	if _, ok := row0[1].(float64); !ok {
		t.Errorf("строка 0 [1] должна быть float64, получили %T: %v", row0[1], row0[1])
	}

	// Строка 1: дочерняя группа «Отдел A» (уровень 1) — с отступом
	row1 := xlsRows[1]
	grpLabel1, _ := row1[0].(string)
	const indent = "  " // 2 пробела на уровень
	if grpLabel1 != indent+"Отдел A" {
		t.Errorf("строка 1 [0] = %q, ожидали %q", grpLabel1, indent+"Отдел A")
	}

	// Строка 2: подытог «Отдел A» (уровень 1) — отступ level+1=2 → 4 пробела
	row2 := xlsRows[2]
	sub2, _ := row2[0].(string)
	const indentL2 = "    " // 4 пробела (уровень 2)
	if sub2 != indentL2+"··· Итого: Отдел A ···" {
		t.Errorf("строка 2 (подытог) [0] = %q, ожидали %q", sub2, indentL2+"··· Итого: Отдел A ···")
	}

	// Строка 3: подытог «Иванов» (уровень 0) — отступ level+1=1 → 2 пробела
	row3 := xlsRows[3]
	sub3, _ := row3[0].(string)
	if sub3 != indent+"··· Итого: Иванов ···" {
		t.Errorf("строка 3 (подытог Иванов) [0] = %q, ожидали %q", sub3, indent+"··· Итого: Иванов ···")
	}

	// Последняя строка: ВСЕГО
	last := xlsRows[len(xlsRows)-1]
	grandLabel, _ := last[0].(string)
	if grandLabel != "ВСЕГО" {
		t.Errorf("последняя строка [0] = %q, ожидали %q", grandLabel, "ВСЕГО")
	}
	if _, ok := last[1].(float64); !ok {
		t.Errorf("последняя строка [1] должна быть float64, получили %T: %v", last[1], last[1])
	}
}

// TestComposedRowsFlat проверяет однуровневую группировку без деталей.
func TestComposedRowsFlat(t *testing.T) {
	rows := []compose.Row{
		{"М": "Иванов", "Сумма": "150"},
		{"М": "Петров", "Сумма": "30"},
	}
	spec := report.Composition{
		Groupings: []string{"М"},
		Measures:  []report.Measure{{Field: "Сумма", Agg: "sum"}},
		Totals:    report.Totals{Grand: true},
	}
	res, _ := compose.Compose(rows, spec, nil)
	headers, xlsRows := composedRows(res, &spec)

	if headers[0] != "М" {
		t.Errorf("headers[0] = %q, ожидали %q", headers[0], "М")
	}
	if headers[1] != "Сумма" {
		t.Errorf("headers[1] = %q, ожидали %q", headers[1], "Сумма")
	}

	// 2 группы + 1 ВСЕГО = 3 строки (без подытогов)
	if len(xlsRows) != 3 {
		t.Fatalf("ожидали 3 строки, получили %d", len(xlsRows))
	}
	last := xlsRows[2]
	if s, _ := last[0].(string); s != "ВСЕГО" {
		t.Errorf("строка ВСЕГО: got %q", s)
	}
}
