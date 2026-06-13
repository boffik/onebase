package storage

import "testing"

// #18: верхняя граница «по дату» для суточного значения — следующий день и «<»
// (включает весь выбранный день на обоих диалектах); с явным временем — «<=».
func TestDateUpperBound(t *testing.T) {
	cases := []struct {
		in    string
		bound string
		op    string
	}{
		{"2026-06-12", "2026-06-13", "<"},
		{"2026-06-30", "2026-07-01", "<"}, // переход месяца
		{"2026-12-31", "2027-01-01", "<"}, // переход года
		{"2026-06-12T10:00:00", "2026-06-12T10:00:00", "<="},
		{"мусор", "мусор", "<="},
	}
	for _, c := range cases {
		b, op := dateUpperBound(c.in)
		if b != c.bound || op != c.op {
			t.Errorf("dateUpperBound(%q) = (%q,%q), ожидалось (%q,%q)", c.in, b, op, c.bound, c.op)
		}
	}
}
