package excel

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestExportList(t *testing.T) {
	cols := []string{"Наименование", "Цена", "Дата"}
	rows := [][]any{
		{"Яблоко", 15.5, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"Банан", 20.0, time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)},
	}

	data, err := ExportList(cols, rows)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Parse the resulting xlsx and verify content
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)

	sheet := "Лист1"
	// Check headers
	a1, _ := f.GetCellValue(sheet, "A1")
	b1, _ := f.GetCellValue(sheet, "B1")
	require.Equal(t, "Наименование", a1)
	require.Equal(t, "Цена", b1)

	// Check first data row
	a2, _ := f.GetCellValue(sheet, "A2")
	require.Equal(t, "Яблоко", a2)

	// Check date formatting
	c2, _ := f.GetCellValue(sheet, "C2")
	require.Equal(t, "01.05.2026", c2)
}

func TestExportListEmpty(t *testing.T) {
	data, err := ExportList([]string{"A", "B"}, nil)
	require.NoError(t, err)
	require.NotEmpty(t, data)
}

func TestWriteList(t *testing.T) {
	cols := []string{"Наименование", "Цена", "Дата"}
	rows := [][]any{
		{"Яблоко", 15.5, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"Банан", 20.0, time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)},
		{"Груша", nil, nil},
	}

	var buf bytes.Buffer
	require.NoError(t, WriteList(&buf, cols, rows))
	require.NotEmpty(t, buf.Bytes())

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	sheet := "Лист1"

	a1, _ := f.GetCellValue(sheet, "A1")
	require.Equal(t, "Наименование", a1)
	c1, _ := f.GetCellValue(sheet, "C1")
	require.Equal(t, "Дата", c1)

	a2, _ := f.GetCellValue(sheet, "A2")
	require.Equal(t, "Яблоко", a2)
	b2, _ := f.GetCellValue(sheet, "B2")
	require.Equal(t, "15.5", b2)
	c2, _ := f.GetCellValue(sheet, "C2")
	require.Equal(t, "01.05.2026", c2)

	// nil-ячейки → пусто
	b4, _ := f.GetCellValue(sheet, "B4")
	require.Equal(t, "", b4)

	rowsRead, err := f.GetRows(sheet)
	require.NoError(t, err)
	require.Len(t, rowsRead, 4) // шапка + 3 строки данных
}

// WriteList (StreamWriter) должен давать те же значения ячеек, что и ExportList,
// — потоковый вариант эквивалентен буферному по содержимому.
func TestWriteListMatchesExportList(t *testing.T) {
	cols := []string{"Товар", "Кол-во", "Сумма"}
	rows := [][]any{
		{"Стол", 3, 1500.0},
		{"Стул", 12, 800.5},
	}
	data, err := ExportList(cols, rows)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, WriteList(&buf, cols, rows))

	fe, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	fw, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)

	sheet := "Лист1"
	for _, ref := range []string{"A1", "B1", "C1", "A2", "B2", "C2", "A3", "B3", "C3"} {
		ve, _ := fe.GetCellValue(sheet, ref)
		vw, _ := fw.GetCellValue(sheet, ref)
		require.Equal(t, ve, vw, "ячейка %s должна совпадать", ref)
	}
}

func TestWriteListEmpty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteList(&buf, []string{"A", "B"}, nil))
	require.NotEmpty(t, buf.Bytes())
}
