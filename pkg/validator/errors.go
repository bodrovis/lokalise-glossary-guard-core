package validator

import (
	"errors"
	"fmt"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func failFastError(
	unit checks.CheckUnit,
	outcome checks.CheckOutcome,
	opts checks.RunOptions,
) error {
	if outcome.Result.Status == checks.Error && opts.HardFailOnErr {
		return fmt.Errorf(
			"fail-fast on ERROR at %q: %s",
			unit.Name(),
			outcome.Result.Message,
		)
	}

	return nil
}

func hardFailError(summary Summary, opts checks.RunOptions) error {
	if !opts.HardFailOnErr || summary.Error == 0 {
		return nil
	}

	msg := firstErrorMessage(summary)
	if msg == "" {
		msg = "one or more checks returned ERROR"
	}

	return errors.New(msg)
}

func firstErrorMessage(summary Summary) string {
	for _, outcome := range summary.Outcomes {
		if outcome.Result.Status == checks.Error && outcome.Result.Message != "" {
			return outcome.Result.Message
		}
	}

	return ""
}
