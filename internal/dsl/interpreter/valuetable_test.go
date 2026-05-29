package interpreter

import "testing"

func newVT(t *testing.T, cols ...string) *ValueTable {
	t.Helper()
	vt := NewValueTable(nil)
	for _, c := range cols {
		vt.addColumn(c)
	}
	return vt
}

func addVTRow(vt *ValueTable, vals map[string]any) {
	row := vt.CallMethod("добавить", nil).(*MapThis)
	for k, v := range vals {
		row.Set(k, v)
	}
}

func TestVT_AddCountTotal(t *testing.T) {
	vt := newVT(t, "Товар", "Сумма")
	addVTRow(vt, map[string]any{"Товар": "Стул", "Сумма": float64(100)})
	addVTRow(vt, map[string]any{"Товар": "Стол", "Сумма": float64(250)})

	if got := vt.CallMethod("количество", nil); got != float64(2) {
		t.Errorf("Количество: got %v", got)
	}
	if got := vt.CallMethod("итог", []any{"Сумма"}); got != float64(350) {
		t.Errorf("Итог: got %v", got)
	}
}

func TestVT_UnloadLoadColumn(t *testing.T) {
	vt := newVT(t, "Сумма")
	addVTRow(vt, map[string]any{"Сумма": float64(10)})
	addVTRow(vt, map[string]any{"Сумма": float64(20)})

	col := vt.CallMethod("выгрузитьколонку", []any{"Сумма"}).(*Array)
	if len(col.items) != 2 || col.items[0] != float64(10) || col.items[1] != float64(20) {
		t.Fatalf("ВыгрузитьКолонку: %v", col.items)
	}
	// Загрузить новые значения в новую колонку.
	vt.CallMethod("загрузитьколонку", []any{&Array{items: []any{float64(1), float64(2)}}, "Цена"})
	price := vt.CallMethod("выгрузитьколонку", []any{"Цена"}).(*Array)
	if price.items[0] != float64(1) || price.items[1] != float64(2) {
		t.Errorf("ЗагрузитьКолонку: %v", price.items)
	}
}

func TestVT_FindAndFindRows(t *testing.T) {
	vt := newVT(t, "Товар", "Цвет")
	addVTRow(vt, map[string]any{"Товар": "Стул", "Цвет": "Красный"})
	addVTRow(vt, map[string]any{"Товар": "Стол", "Цвет": "Красный"})

	row := vt.CallMethod("найти", []any{"Стол", "Товар"})
	if row == nil || row.(*MapThis).Get("Цвет") != "Красный" {
		t.Errorf("Найти: %v", row)
	}
	if vt.CallMethod("найти", []any{"НетТакого", "Товар"}) != nil {
		t.Error("Найти несуществующего → nil")
	}

	filt := newStruct([]any{"Цвет", "Красный"})
	rows := vt.CallMethod("найтистроки", []any{filt}).(*Array)
	if len(rows.items) != 2 {
		t.Errorf("НайтиСтроки: ожидалось 2, got %d", len(rows.items))
	}
}

func TestVT_Sort(t *testing.T) {
	vt := newVT(t, "Сумма")
	for _, v := range []float64{30, 10, 20} {
		addVTRow(vt, map[string]any{"Сумма": v})
	}
	vt.CallMethod("сортировать", []any{"Сумма Убыв"})
	got := []float64{}
	for _, r := range vt.rows {
		f, _ := toFloat(r["сумма"])
		got = append(got, f)
	}
	if got[0] != 30 || got[1] != 20 || got[2] != 10 {
		t.Errorf("Сортировать убыв: %v", got)
	}
}

func TestVT_Collapse(t *testing.T) {
	vt := newVT(t, "Товар", "Количество")
	addVTRow(vt, map[string]any{"Товар": "Стул", "Количество": float64(3)})
	addVTRow(vt, map[string]any{"Товар": "Стол", "Количество": float64(5)})
	addVTRow(vt, map[string]any{"Товар": "Стул", "Количество": float64(2)})

	vt.CallMethod("свернуть", []any{"Товар", "Количество"})
	if got := vt.CallMethod("количество", nil); got != float64(2) {
		t.Fatalf("после Свернуть ожидалось 2 строки, got %v", got)
	}
	// Стул: 3+2=5
	row := vt.CallMethod("найти", []any{"Стул", "Товар"}).(*MapThis)
	if got := row.Get("Количество"); got != float64(5) {
		t.Errorf("Свернуть сумма Стул: got %v", got)
	}
}
