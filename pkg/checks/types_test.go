package checks_test

import (
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
