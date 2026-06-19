package semicolon_separator

import (
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-semicolon-separators"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureSemicolonSeparators,
		checks.WithFailFast(),
		checks.WithPriority(6),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runEnsureSemicolonSeparators(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateSemicolonSeparated,
		Fix:              fixToSemicolonsIfConsistent,
		PassMsg:          "file uses semicolons as separators",
		FixedMsg:         "converted separators to semicolons",
		AppliedMsg:       "auto-fix applied: converted separators to semicolons",
		StillBadMsg:      "auto-fix attempted but file is still not cleanly semicolon-separated",
		StatusAfterFixed: checks.Pass,
	})
}

func validateSemicolonSeparated(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return cancelledValidation(err)
	}

	dataBytes := checks.StripUTF8BOM(a.Data)
	if checks.IsBlankUnicode(dataBytes) {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot detect separators: no usable content",
		}
	}

	report, err := detectSeparators(ctx, dataBytes)
	if err != nil {
		return cancelledValidation(err)
	}

	if report.semicolonOK {
		return checks.ValidationResult{OK: true}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: separatorFailureMessage(report),
	}
}
