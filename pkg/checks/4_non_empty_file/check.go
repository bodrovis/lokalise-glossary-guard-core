package non_empty_file

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-not-empty"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureNotEmpty,
		checks.WithFailFast(),
		checks.WithPriority(4),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runEnsureNotEmpty(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateNotEmpty,
		Fix:              fixAddHeaderIfEmpty,
		PassMsg:          "file is not empty",
		FixedMsg:         "inserted CSV header",
		AppliedMsg:       "auto-fix applied (inserted CSV header)",
		StatusAfterFixed: checks.Pass,
	})
}

func validateNotEmpty(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	if len(strings.TrimSpace(string(a.Data))) == 0 {
		return checks.ValidationResult{OK: false, Msg: "empty file: no data"}
	}
	return checks.ValidationResult{OK: true}
}
