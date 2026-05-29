package interpreter

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

func builtinReadJSON(args []any, file string, line int) (any, error) {
	if len(args) == 0 {
		panic(userError{Msg: "ПрочитатьJSON: ожидается 1 аргумент"})
	}
	text := fmt.Sprintf("%v", args[0])
	var raw any
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		panic(userError{Msg: "ПрочитатьJSON: " + err.Error()})
	}
	return jsonToValue(raw), nil
}

func jsonToValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := &Map{}
		for k, val := range x {
			m.CallMethod("вставить", []any{k, jsonToValue(val)})
		}
		return m
	case []any:
		a := &Array{}
		for _, item := range x {
			a.items = append(a.items, jsonToValue(item))
		}
		return a
	case float64:
		// json.Unmarshal returns all numbers as float64
		if x == math.Trunc(x) && !math.IsInf(x, 0) && !math.IsNaN(x) {
			return int64(x)
		}
		return x
	case bool, string:
		return x
	default:
		return nil // null → Неопределено
	}
}

func builtinWriteJSON(args []any, file string, line int) (any, error) {
	if len(args) == 0 {
		return "null", nil
	}
	data, err := json.Marshal(valueToJSON(args[0]))
	if err != nil {
		panic(userError{Msg: "ЗаписатьJSON: " + err.Error()})
	}
	return string(data), nil
}

// valueToJSON рекурсивно конвертирует DSL-значение в JSON-совместимый тип.
func valueToJSON(v any) any {
	switch x := v.(type) {
	case *Map:
		obj := make(map[string]any, len(x.keys))
		for i, k := range x.keys {
			obj[fmt.Sprintf("%v", k)] = valueToJSON(x.vals[i])
		}
		return obj
	case *Struct:
		obj := make(map[string]any, len(x.keys))
		for _, k := range x.keys {
			obj[k] = valueToJSON(x.vals[k])
		}
		return obj
	case *Array:
		items := make([]any, len(x.items))
		for i, item := range x.items {
			items[i] = valueToJSON(item)
		}
		return items
	case decimal.Decimal:
		// json.Number маршалится как число без кавычек. По умолчанию shopspring
		// сериализует decimal строкой ("30"), что ломает совместимость с JSON-числами.
		return json.Number(x.String())
	default:
		return v // string, float64, int64, bool, nil — маршалятся напрямую
	}
}
