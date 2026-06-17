package compose

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFormatNumber(t *testing.T) {
	cases := []struct{ in, format, want string }{
		{"1234567.5", "#,##0.00", "1 234 567,50"},
		{"1234567", "#,##0", "1 234 567"},
		{"0.1234", "0.0%", "12,3%"},
		{"42", "", "42"},
		{"-333.34", "#,##0.00", "-333,34"},
	}
	for _, c := range cases {
		d, _ := decimal.NewFromString(c.in)
		if got := FormatNumber(d, c.format); got != c.want {
			t.Errorf("FormatNumber(%s,%q)=%q want %q", c.in, c.format, got, c.want)
		}
	}
}
