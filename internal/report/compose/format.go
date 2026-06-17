package compose

import (
	"strings"

	"github.com/shopspring/decimal"
)

// FormatNumber форматирует decimal.Decimal по строке формата в RU-локали.
//
// Поддерживаемые форматы (подмножество):
//   - ""       → сырое d.String() без изменений
//   - "#,##0.00" → разрядность тысяч (неразрывный пробел) + 2 знака, запятая
//   - "#,##0"   → целое с разрядкой
//   - "0.0%"   → процент (×100), 1 знак, суффикс %
//
// Правила разбора:
//   - наличие "%" в конце → умножить значение на 100, добавить суффикс "%"
//   - наличие "," до точки → группировать целую часть по 3 знака (неразрывный пробел " ")
//   - число знаков после "." = количество символов 0/# после точки (0 если точки нет)
//   - десятичный разделитель — ","
func FormatNumber(d decimal.Decimal, format string) string {
	if format == "" {
		return d.String()
	}

	// Определяем суффикс %
	percent := strings.HasSuffix(format, "%")

	// Разделяем формат на часть до/после десятичной точки
	intPart := format
	fracPart := ""
	if idx := strings.Index(format, "."); idx >= 0 {
		intPart = format[:idx]
		fracPart = format[idx+1:]
		// Убираем суффикс % из fracPart
		fracPart = strings.TrimSuffix(fracPart, "%")
	} else {
		intPart = strings.TrimSuffix(intPart, "%")
	}

	// Нужна ли разрядность?
	grouped := strings.Contains(intPart, ",")

	// Число знаков после точки
	decimals := int32(0)
	for _, ch := range fracPart {
		if ch == '0' || ch == '#' {
			decimals++
		}
	}

	// Применяем процент: умножаем на 100
	if percent {
		d = d.Mul(decimal.NewFromInt(100))
	}

	// Округляем до нужного количества знаков
	d = d.Round(decimals)

	// Получаем строку с фиксированным числом знаков (точка как разделитель)
	s := d.StringFixed(decimals)

	// Разбираем знак, целую и дробную части
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}

	dotIdx := strings.Index(s, ".")
	intStr := s
	fracStr := ""
	if dotIdx >= 0 {
		intStr = s[:dotIdx]
		fracStr = s[dotIdx+1:]
	}

	// Группируем целую часть по 3 знака справа налево (неразрывный пробел)
	if grouped {
		intStr = groupDigits(intStr)
	}

	// Собираем результат
	var result strings.Builder
	result.WriteString(sign)
	result.WriteString(intStr)
	if decimals > 0 {
		result.WriteString(",")
		result.WriteString(fracStr)
	}
	if percent {
		result.WriteString("%")
	}
	return result.String()
}

// groupDigits вставляет неразрывный пробел каждые 3 цифры справа налево.
func groupDigits(s string) string {
	if len(s) <= 3 {
		return s
	}
	// Разбиваем справа налево
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, " ")
}
