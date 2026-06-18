package validator

import (
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func shouldStop(unit checks.CheckUnit, outcome checks.CheckOutcome) bool {
	if !unit.FailFast() {
		return false
	}

	switch outcome.Result.Status {
	case checks.Fail, checks.Error:
		return true
	default:
		return false
	}
}
