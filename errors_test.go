package sdk_test

import (
	"errors"
	"fmt"
	"testing"

	sdk "github.com/DouDOU-start/airgate-sdk"
)

func TestSentinelErrorsAreDistinct(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotSupported", sdk.ErrNotSupported},
		{"ErrInvalidCredentials", sdk.ErrInvalidCredentials},
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a.err, b.err) {
				t.Errorf("%s should not match %s via errors.Is", a.name, b.name)
			}
			if a.err.Error() == b.err.Error() {
				t.Errorf("%s and %s have identical message %q", a.name, b.name, a.err.Error())
			}
		}
	}
}

func TestSentinelErrorMessages(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{sdk.ErrNotSupported, "not supported"},
		{sdk.ErrInvalidCredentials, "invalid credentials"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestErrorsIsWithWrapping(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotSupported", sdk.ErrNotSupported},
		{"ErrInvalidCredentials", sdk.ErrInvalidCredentials},
	}

	for _, tc := range sentinels {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("something went wrong: %w", tc.err)

			if !errors.Is(wrapped, tc.err) {
				t.Errorf("errors.Is(wrapped, %s) should be true", tc.name)
			}

			doubleWrapped := fmt.Errorf("outer: %w", wrapped)
			if !errors.Is(doubleWrapped, tc.err) {
				t.Errorf("errors.Is(doubleWrapped, %s) should be true", tc.name)
			}

			if got := wrapped.Error(); got != "something went wrong: "+tc.err.Error() {
				t.Errorf("unexpected wrapped message: %q", got)
			}
		})
	}
}
