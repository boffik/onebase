package launcher

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func formReq(values url.Values) *http.Request {
	r, _ := http.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	return r
}

func TestParseMapForm(t *testing.T) {
	r := formReq(url.Values{
		"titles.en":         {"Counterparty"},
		"titles.de":         {"  Geschäftspartner  "}, // trimmed
		"titles.fr":         {"   "},                   // empty after trim → skip
		"titles.ru":         {"Контрагент"},            // base lang → skip
		"field.0.titles.en": {"Name"},                  // другой префикс → не попадает
	})

	got := parseMapForm(r, "titles")
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 перевода, получили %d: %v", len(got), got)
	}
	if got["en"] != "Counterparty" || got["de"] != "Geschäftspartner" {
		t.Errorf("неверные значения: %v", got)
	}
	if _, ok := got["ru"]; ok {
		t.Error("базовый ru не должен попадать в карту переводов")
	}
	if _, ok := got["fr"]; ok {
		t.Error("пустой перевод не должен попадать в карту")
	}

	nested := parseMapForm(r, "field.0.titles")
	if len(nested) != 1 || nested["en"] != "Name" {
		t.Errorf("вложенный префикс: %v", nested)
	}

	if parseMapForm(formReq(url.Values{}), "titles") != nil {
		t.Error("пустой результат должен быть nil")
	}
}
