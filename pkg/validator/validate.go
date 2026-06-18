package validator

import (
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// Validate runs all registered checks in sorted order and returns a summary.
func Validate(
	ctx context.Context,
	filePath string,
	data []byte,
	langs []string,
	opts checks.RunOptions,
) (Summary, error) {
	state := newRunState(filePath, data, langs)

	for _, unit := range checks.ListSorted() {
		if err := contextError(ctx); err != nil {
			state.markContextEarlyExit()
			return state.summary, err
		}

		outcome := state.runCheck(ctx, unit, opts)

		if shouldStop(unit, outcome) {
			state.markEarlyExit(unit, outcome)
			return state.summary, failFastError(unit, outcome, opts)
		}
	}

	if err := hardFailError(state.summary, opts); err != nil {
		return state.summary, err
	}

	return state.summary, nil
}
