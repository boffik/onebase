package interpreter

import "fmt"

// ─── Array (Массив) ───────────────────────────────────────────────────────────

type Array struct {
	items []any
}

func (a *Array) CallMethod(name string, args []any) any {
	switch name {
	case "Добавить", "Add":
		if len(args) > 0 {
			a.items = append(a.items, args[0])
		}
	case "Количество", "Count":
		return float64(len(a.items))
	case "Получить", "Get":
		idx := int(floatArg(args, 0))
		if idx >= 0 && idx < len(a.items) {
			return a.items[idx]
		}
	case "Удалить", "Delete":
		idx := int(floatArg(args, 0))
		if idx >= 0 && idx < len(a.items) {
			a.items = append(a.items[:idx], a.items[idx+1:]...)
		}
	case "Очистить", "Clear":
		a.items = nil
	case "Вставить", "Insert":
		if len(args) >= 2 {
			idx := int(floatArg(args, 0))
			val := args[1]
			if idx < 0 {
				idx = 0
			}
			if idx >= len(a.items) {
				a.items = append(a.items, val)
			} else {
				a.items = append(a.items, nil)
				copy(a.items[idx+1:], a.items[idx:])
				a.items[idx] = val
			}
		}
	}
	return nil
}

func (a *Array) Index(i int) any {
	if i >= 0 && i < len(a.items) {
		return a.items[i]
	}
	return nil
}

func (a *Array) SetIndex(i int, val any) {
	if i >= 0 && i < len(a.items) {
		a.items[i] = val
	}
}

func (a *Array) Iterate() []any { return a.items }

// ─── Struct (Структура) ───────────────────────────────────────────────────────

type Struct struct {
	keys []string
	vals map[string]any
}

func newStruct(args []any) *Struct {
	s := &Struct{vals: make(map[string]any)}
	if len(args) == 0 {
		return s
	}
	// args[0] — строка с именами полей через запятую
	fields := splitComma(strArg(args, 0))
	for i, f := range fields {
		s.keys = append(s.keys, f)
		if i+1 < len(args) {
			s.vals[f] = args[i+1]
		} else {
			s.vals[f] = nil
		}
	}
	return s
}

func (s *Struct) Get(field string) any    { return s.vals[field] }
func (s *Struct) Set(field string, v any) { s.vals[field] = v }

func (s *Struct) CallMethod(name string, args []any) any {
	switch name {
	case "Вставить", "Insert":
		if len(args) >= 1 {
			key := strArg(args, 0)
			if _, exists := s.vals[key]; !exists {
				s.keys = append(s.keys, key)
			}
			var val any
			if len(args) >= 2 {
				val = args[1]
			}
			s.vals[key] = val
		}
	case "Удалить", "Delete":
		key := strArg(args, 0)
		delete(s.vals, key)
		for i, k := range s.keys {
			if k == key {
				s.keys = append(s.keys[:i], s.keys[i+1:]...)
				break
			}
		}
	case "Количество", "Count":
		return float64(len(s.keys))
	case "Свойство", "Property":
		key := strArg(args, 0)
		v, ok := s.vals[key]
		if !ok {
			return nil
		}
		return v
	}
	return nil
}

// ─── Map / Соответствие ───────────────────────────────────────────────────────

type Map struct {
	keys []any
	vals []any
}

func (m *Map) findIdx(key any) int {
	ks := fmt.Sprintf("%v", key)
	for i, k := range m.keys {
		if fmt.Sprintf("%v", k) == ks {
			return i
		}
	}
	return -1
}

func (m *Map) CallMethod(name string, args []any) any {
	switch name {
	case "Вставить", "Insert":
		if len(args) >= 1 {
			key := args[0]
			var val any
			if len(args) >= 2 {
				val = args[1]
			}
			if idx := m.findIdx(key); idx >= 0 {
				m.vals[idx] = val
			} else {
				m.keys = append(m.keys, key)
				m.vals = append(m.vals, val)
			}
		}
	case "Получить", "Get":
		if len(args) >= 1 {
			if idx := m.findIdx(args[0]); idx >= 0 {
				return m.vals[idx]
			}
		}
	case "Удалить", "Delete":
		if len(args) >= 1 {
			if idx := m.findIdx(args[0]); idx >= 0 {
				m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
				m.vals = append(m.vals[:idx], m.vals[idx+1:]...)
			}
		}
	case "Количество", "Count":
		return float64(len(m.keys))
	case "Очистить", "Clear":
		m.keys = nil
		m.vals = nil
	}
	return nil
}

// ─── KeyValue — элемент итерации по Соответствию ─────────────────────────────

type KeyValue struct {
	Key   any
	Value any
}

func (kv *KeyValue) Get(field string) any {
	switch field {
	case "Ключ", "Key":
		return kv.Key
	case "Значение", "Value":
		return kv.Value
	}
	return nil
}

func (kv *KeyValue) Set(field string, val any) {}

// ─── helpers ─────────────────────────────────────────────────────────────────

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
