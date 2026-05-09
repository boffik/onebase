package debugger

import (
	"fmt"
	"strings"
)

// This file kept minimal — expression evaluation is done via DSL parser+interpreter.
// See controller.go ActiveSession.Evaluate() for the actual mechanism.

// ParseUserValue parses a user-entered value as the specified type (for variable editing)
func ParseUserValue(valueStr, targetType string) (any, error) {
	switch targetType {
	case "Число":
		var f float64
		_, err := fmt.Sscanf(valueStr, "%f", &f)
		return f, err
	case "Строка":
		return valueStr, nil
	case "Булево":
		lower := strings.ToLower(strings.TrimSpace(valueStr))
		if lower == "истина" || lower == "true" || lower == "1" {
			return true, nil
		}
		if lower == "ложь" || lower == "false" || lower == "0" {
			return false, nil
		}
		return nil, fmt.Errorf("invalid boolean: %s", valueStr)
	default:
		return nil, fmt.Errorf("unsupported type: %s", targetType)
	}
}
