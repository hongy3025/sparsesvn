package executor

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors_Is(t *testing.T) {
	cases := []struct {
		name      string
		wrapped   error
		sentinel  error
		wantMatch bool
	}{
		{"URLMismatch", fmt.Errorf("%w: details", ErrURLMismatch), ErrURLMismatch, true},
		{"URLRequired", fmt.Errorf("%w: details", ErrURLRequired), ErrURLRequired, true},
		{"ConfigInvalid", fmt.Errorf("%w: details", ErrConfigInvalid), ErrConfigInvalid, true},
		{"SvnFailed", fmt.Errorf("%w: details", ErrSvnFailed), ErrSvnFailed, true},
		{"StateCorrupt", fmt.Errorf("%w: details", ErrStateCorrupt), ErrStateCorrupt, true},
		{"StateFutureVer", fmt.Errorf("%w: details", ErrStateFutureVer), ErrStateFutureVer, true},
		{"URLMismatch_no_match", ErrURLMismatch, ErrURLRequired, false},
		{"SvnFailed_no_match", ErrSvnFailed, ErrConfigInvalid, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := errors.Is(tc.wrapped, tc.sentinel)
			if got != tc.wantMatch {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tc.wrapped, tc.sentinel, got, tc.wantMatch)
			}
		})
	}
}

func TestSentinelErrors_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	wrapped := fmt.Errorf("%w: %w", ErrSvnFailed, inner)

	if !errors.Is(wrapped, ErrSvnFailed) {
		t.Error("expected errors.Is(wrapped, ErrSvnFailed) = true")
	}
	if !errors.Is(wrapped, inner) {
		t.Error("expected errors.Is(wrapped, inner) = true")
	}
}
