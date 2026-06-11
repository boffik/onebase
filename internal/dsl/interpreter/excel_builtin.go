package interpreter

import (
	"encoding/base64"
	"fmt"

	"github.com/ivantit66/onebase/internal/excel"
	"github.com/ivantit66/onebase/internal/i18n/i18nerr"
)

func init() {
	builtins["выгрузитьвexcel"] = builtinExportExcel
	builtins["exportexcel"] = builtinExportExcel
}

// builtinExportExcel(data, title)
// data — Массив массивов; первый подмассив — заголовки, остальные — строки данных.
// Возвращает base64-строку содержимого xlsx-файла.
func builtinExportExcel(args []any, file string, line int) (any, error) {
	if len(args) < 1 {
		return nil, i18nerr.New("ВыгрузитьВExcel: ожидается аргумент Данные (Массив)")
	}

	outerArr, ok := args[0].(*Array)
	if !ok {
		return nil, i18nerr.New("ВыгрузитьВExcel: аргумент Данные должен быть Массивом")
	}
	if len(outerArr.items) < 1 {
		return "", nil
	}

	// First row → column headers
	firstRow, ok := outerArr.items[0].(*Array)
	if !ok {
		return nil, i18nerr.New("ВыгрузитьВExcel: первая строка (заголовки) должна быть Массивом")
	}
	cols := make([]string, len(firstRow.items))
	for i, v := range firstRow.items {
		cols[i] = fmt.Sprintf("%v", v)
	}

	// Data rows
	rows := make([][]any, 0, len(outerArr.items)-1)
	for _, rowVal := range outerArr.items[1:] {
		rowArr, ok := rowVal.(*Array)
		if !ok {
			continue
		}
		cells := make([]any, len(rowArr.items))
		copy(cells, rowArr.items)
		rows = append(rows, cells)
	}

	data, err := excel.ExportList(cols, rows)
	if err != nil {
		return nil, i18nerr.Wrapf(err, "ВыгрузитьВExcel")
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
