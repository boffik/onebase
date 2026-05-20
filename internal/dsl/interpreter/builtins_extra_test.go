package interpreter

import "testing"

// ПустаяСсылка — узкий предикат именно для ссылочных значений.
func TestPustayaSsylka_Nil(t *testing.T) {
	if !isEmptyRefVal(nil) {
		t.Error("nil должен быть пустой ссылкой")
	}
}

func TestPustayaSsylka_EmptyString(t *testing.T) {
	if !isEmptyRefVal("") {
		t.Error(`"" должен быть пустой ссылкой`)
	}
}

func TestPustayaSsylka_ZeroUUID(t *testing.T) {
	if !isEmptyRefVal("00000000-0000-0000-0000-000000000000") {
		t.Error("uuid.Nil должен быть пустой ссылкой")
	}
}

func TestPustayaSsylka_ValidUUID(t *testing.T) {
	if isEmptyRefVal("11111111-1111-1111-1111-111111111111") {
		t.Error("обычный UUID не пустая ссылка")
	}
}

func TestPustayaSsylka_EmptyRef(t *testing.T) {
	r := &Ref{UUID: "", Name: ""}
	if !isEmptyRefVal(r) {
		t.Error("*Ref с пустым UUID — пустая ссылка")
	}
	r2 := &Ref{UUID: "11111111-1111-1111-1111-111111111111", Name: "X"}
	if isEmptyRefVal(r2) {
		t.Error("*Ref с UUID — НЕ пустая ссылка")
	}
}

// 0 / Ложь — это не «пустая ссылка», в отличие от Пустая(x).
func TestPustayaSsylka_NotConfusedWithBlank(t *testing.T) {
	if isEmptyRefVal(float64(0)) {
		t.Error("0 не должен считаться пустой ссылкой")
	}
	if isEmptyRefVal(false) {
		t.Error("Ложь не должна считаться пустой ссылкой")
	}
}
