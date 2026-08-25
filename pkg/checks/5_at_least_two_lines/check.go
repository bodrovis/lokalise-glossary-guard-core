package at_least_two_lines

import (
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-at-least-two-lines"

const (
	requiredNonEmptyLines = 2
	maxScannedLineSize    = 16 << 20
)

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureAtLeastTwoLines,
		checks.WithFailFast(),
		checks.WithPriority(5),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

// runEnsureAtLeastTwoLines — entry point for the check.
// There is no auto-fix for this one.
func runEnsureAtLeastTwoLines(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:     checkName,
		Validate: validateAtLeastTwoLines,
		PassMsg:  "file has at least two lines (header + data)",
		FailAs:   checks.Fail,
	})
}

func validateAtLeastTwoLines(
	ctx context.Context,
	a checks.Artifact,
) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.CancelledValidation(err)
	}

	data := checks.StripUTF8BOM(a.Data)

	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  false,
			Msg: "empty file: expected header and at least one data row",
		}
	}

	ok, err := hasAtLeastNonEmptyLines(
		ctx,
		data,
		requiredNonEmptyLines,
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return checks.CancelledValidation(ctxErr)
		}

		return checks.ValidationResult{
			OK:  false,
			Msg: "failed to read file",
			Err: err,
		}
	}

	if ok {
		return checks.ValidationResult{
			OK:  true,
			Msg: "has ≥2 lines",
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: "expected at least two non-empty lines (header + one data row)",
	}
}

func hasAtLeastNonEmptyLines(
	ctx context.Context,
	data []byte,
	want int,
) (bool, error) {
	scanner := checks.NewLineScanner(data, maxScannedLineSize)

	nonEmpty := 0

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		if isNonEmptyCSVLine(scanner.Bytes()) {
			nonEmpty++
		}

		if nonEmpty >= want {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}

func isNonEmptyCSVLine(line []byte) bool {
	return !checks.IsBlankUnicode(
		checks.TrimTrailingCR(line),
	)
}
