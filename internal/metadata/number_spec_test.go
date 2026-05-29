package metadata

import "testing"

func TestParseNumberSpec(t *testing.T) {
	tests := []struct {
		in        string
		wantLen   int
		wantScale int
		wantOK    bool
	}{
		{"number(10,2)", 10, 2, true},
		{"decimal(15,2)", 15, 2, true},
		{"decimal(15)", 15, 0, true},
		{"number( 8 , 3 )", 8, 3, true},
		{"number", 0, 0, false},
		{"string(40)", 0, 0, false},
		{"date", 0, 0, false},
		{"number(abc,2)", 0, 0, false},
	}
	for _, tt := range tests {
		l, s, ok := parseNumberSpec(tt.in)
		if ok != tt.wantOK || l != tt.wantLen || s != tt.wantScale {
			t.Errorf("parseNumberSpec(%q) = (%d,%d,%v), want (%d,%d,%v)",
				tt.in, l, s, ok, tt.wantLen, tt.wantScale, tt.wantOK)
		}
	}
}

func TestParseField_NumberSpec(t *testing.T) {
	f := parseField(rawField{Name: "Цена", Type: "number(10,2)"})
	if f.Type != FieldTypeNumber {
		t.Errorf("Type = %q, want number", f.Type)
	}
	if f.Length != 10 || f.Scale != 2 {
		t.Errorf("Length,Scale = %d,%d, want 10,2", f.Length, f.Scale)
	}

	// Голый number — без разрядности.
	f2 := parseField(rawField{Name: "Кол", Type: "number"})
	if f2.Type != FieldTypeNumber || f2.Length != 0 || f2.Scale != 0 {
		t.Errorf("plain number: Type=%q Length=%d Scale=%d", f2.Type, f2.Length, f2.Scale)
	}
}
