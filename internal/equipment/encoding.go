package equipment

import (
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// deviceEncoder возвращает кодировщик текста в однобайтовую кодировку, понятную
// устройству. Российские VFD-дисплеи и термопринтеры обычно ждут CP866 для
// кириллицы; UTF-8 на таком железе даёт «кракозябры». Управляющие байты ASCII
// (<0x80) во всех этих кодировках совпадают, поэтому кодируется только текст, а
// командные последовательности (ESC @, CR, …) остаются как есть.
//
//	"cp866"/"866"/"ibm866" → CP866
//	"utf8"/"utf-8"/пусто    → как есть
func deviceEncoder(name string) func(string) []byte {
	if normEncoding(name) == "cp866" {
		return encodeCP866
	}
	return func(s string) []byte { return []byte(s) }
}

// normEncoding приводит имя кодировки к канону: "CP-866"/"866"/"IBM866" → "cp866".
func normEncoding(name string) string {
	n := strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(name)))
	switch n {
	case "cp866", "866", "ibm866", "dos", "oem866":
		return "cp866"
	default:
		return "utf8"
	}
}

// encodeCP866 кодирует строку в CP866; символы вне кодировки заменяются на '?'.
func encodeCP866(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if b, ok := charmap.CodePage866.EncodeRune(r); ok {
			out = append(out, b)
		} else {
			out = append(out, '?')
		}
	}
	return out
}
