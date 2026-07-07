package storage

import (
	"errors"
	"testing"
)

func TestIsLockTimeoutErr(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("ERROR: canceling statement due to lock timeout (SQLSTATE 55P03)"), true},
		{errors.New("Lock timeout while waiting"), true},
		{errors.New("connection refused"), false},
	}
	for _, c := range cases {
		if got := isLockTimeoutErr(c.err); got != c.want {
			t.Errorf("isLockTimeoutErr(%q) = %v, want %v", c.err, got, c.want)
		}
	}
}
