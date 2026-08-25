package checks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

var _ error = checks.ErrNoFix

func TestErrNoFix_Error(t *testing.T) {
	if checks.ErrNoFix == nil {
		t.Fatal("ErrNoFix is nil")
	}

	const want = "no fix implemented for this check"

	if got := checks.ErrNoFix.Error(); got != want {
		t.Fatalf("ErrNoFix.Error() = %q, want %q", got, want)
	}
}

func TestCancelledValidation(t *testing.T) {
	err := context.Canceled

	got := checks.CancelledValidation(err)

	if got.OK {
		t.Fatal("OK = true, want false")
	}

	if got.Msg != "validation cancelled" {
		t.Fatalf(
			"Msg = %q, want %q",
			got.Msg,
			"validation cancelled",
		)
	}

	if !errors.Is(got.Err, context.Canceled) {
		t.Fatalf(
			"Err = %v, want context.Canceled",
			got.Err,
		)
	}
}
