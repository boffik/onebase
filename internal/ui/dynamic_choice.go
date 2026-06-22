package ui

import (
	"fmt"

	"github.com/ivantit66/onebase/internal/dsl/interpreter"
)

// choiceListItem — один пункт динамического списка значений, сформированного
// обработчиком события НачалоВыбора элемента ПолеСписка через билтин
// ДобавитьЗначениеСписка. Аналог декларативного FormChoice, но строится кодом.
type choiceListItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// newChoiceListBuiltin возвращает билтин ДобавитьЗначениеСписка(значение[,
// представление]) — копит пункты в sink. После Run они уходят в ответ как
// choiceList, и клиент заполняет ими <select> элемента ПолеСписка (динамический
// список значений, аналог 1С «заполнить СписокВыбора в НачалоВыбора»).
// Представление по умолчанию равно строковому значению.
func newChoiceListBuiltin(sink *[]choiceListItem) interpreter.BuiltinFunc {
	return interpreter.BuiltinFunc(func(args []any, _ string, _ int) (any, error) {
		if sink == nil || len(args) == 0 || args[0] == nil {
			return nil, nil
		}
		val := fmt.Sprint(args[0])
		label := val
		if len(args) > 1 && args[1] != nil {
			if s := fmt.Sprint(args[1]); s != "" {
				label = s
			}
		}
		*sink = append(*sink, choiceListItem{Value: val, Label: label})
		return nil, nil
	})
}
