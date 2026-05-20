package interpreter

import "testing"

// ЧислоПрописью должно правильно склонять рубли и копейки.
func TestAmountInWords_Basic(t *testing.T) {
	cases := []struct {
		amount float64
		want   string
	}{
		{0, "Ноль рублей 00 копеек"},
		{1, "Один рубль 00 копеек"},
		{2, "Два рубля 00 копеек"},
		{5, "Пять рублей 00 копеек"},
		{10, "Десять рублей 00 копеек"},
		{11, "Одиннадцать рублей 00 копеек"},
		{21, "Двадцать один рубль 00 копеек"},
		{22, "Двадцать два рубля 00 копеек"},
		{25, "Двадцать пять рублей 00 копеек"},
		{100, "Сто рублей 00 копеек"},
		{101, "Сто один рубль 00 копеек"},
		{111, "Сто одиннадцать рублей 00 копеек"},
		{1000, "Одна тысяча рублей 00 копеек"},
		{1001, "Одна тысяча один рубль 00 копеек"},
		{1234.56, "Одна тысяча двести тридцать четыре рубля 56 копеек"},
		{2_000_000, "Два миллиона рублей 00 копеек"},
		{1_234_567.89, "Один миллион двести тридцать четыре тысячи пятьсот шестьдесят семь рублей 89 копеек"},
	}
	for _, c := range cases {
		got := AmountInWords(c.amount, "rub")
		if got != c.want {
			t.Errorf("AmountInWords(%v) = %q, want %q", c.amount, got, c.want)
		}
	}
}

// округление копеек должно вести себя устойчиво к float-погрешности.
func TestAmountInWords_KopeckRounding(t *testing.T) {
	cases := []struct {
		amount float64
		want   string
	}{
		{1.999, "Два рубля 00 копеек"}, // 199.9 → 200 → +1 рубль (overflow)
		{1.01, "Один рубль 01 копейка"},
		{1.50, "Один рубль 50 копеек"},
		{0.05, "Ноль рублей 05 копеек"},
		{0.50, "Ноль рублей 50 копеек"},
		{-1.50, "Минус один рубль 50 копеек"},
	}
	for _, c := range cases {
		got := AmountInWords(c.amount, "rub")
		if got != c.want {
			t.Errorf("AmountInWords(%v) = %q, want %q", c.amount, got, c.want)
		}
	}
}

func TestPluralRu(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{1, "рубль"},
		{2, "рубля"},
		{5, "рублей"},
		{11, "рублей"},
		{12, "рублей"},
		{19, "рублей"},
		{21, "рубль"},
		{22, "рубля"},
		{25, "рублей"},
		{101, "рубль"},
		{111, "рублей"},
		{122, "рубля"},
	}
	for _, c := range cases {
		got := pluralRu(c.n, "рубль", "рубля", "рублей")
		if got != c.want {
			t.Errorf("pluralRu(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestAmountInWords_USD(t *testing.T) {
	got := AmountInWords(1234.56, "usd")
	want := "Одна тысяча двести тридцать четыре доллара 56 центов"
	if got != want {
		t.Errorf("AmountInWords USD = %q, want %q", got, want)
	}
}

func TestAmountInWords_FeminineThousand(t *testing.T) {
	// В русском «тысяча» — женский род, поэтому «одна тысяча», «две тысячи».
	cases := []struct {
		amount float64
		want   string
	}{
		{1000, "Одна тысяча рублей 00 копеек"},
		{2000, "Две тысячи рублей 00 копеек"},
		{5000, "Пять тысяч рублей 00 копеек"},
		{21000, "Двадцать одна тысяча рублей 00 копеек"},
	}
	for _, c := range cases {
		got := AmountInWords(c.amount, "rub")
		if got != c.want {
			t.Errorf("AmountInWords(%v) = %q, want %q", c.amount, got, c.want)
		}
	}
}
