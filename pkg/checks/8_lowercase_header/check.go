package lowercase_header

import (
	"context"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-lowercase-header"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureLowercaseHeader,
		checks.WithFailFast(),
		checks.WithPriority(8),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runEnsureLowercaseHeader(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateLowercaseHeader,
		Fix:              fixLowercaseHeader,
		FailAs:           checks.Warn,
		PassMsg:          "header service columns are already lowercase",
		FixedMsg:         "normalized header service columns to lowercase",
		AppliedMsg:       "auto-fix applied: normalized header service columns to lowercase",
		StatusAfterFixed: checks.Pass,
		StillBadMsg:      "header normalized but some service columns are still not lowercase",
	})
}

func validateLowercaseHeader(
	ctx context.Context,
	a checks.Artifact,
) checks.ValidationResult {
	header, res, ok := checks.ReadSemicolonHeader(
		ctx,
		a,
		"cannot check header: no usable content",
	)
	if !ok {
		return res
	}

	bad, err := findNonLowercaseServiceHeaderColumns(ctx, header)
	if err != nil {
		return checks.CancelledValidation(err)
	}

	if len(bad) > 0 {
		return checks.ValidationResult{
			OK: false,
			Msg: "some service columns in header are not lowercase at positions: " +
				strings.Join(bad, ", "),
		}
	}

	return checks.ValidationResult{
		OK:  true,
		Msg: "header service columns are already lowercase",
	}
}

func findNonLowercaseServiceHeaderColumns(ctx context.Context, header []string) ([]string, error) {
	var bad []string

	for i, col := range header {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if isNonLowercaseKnownHeader(col) {
			bad = append(bad, strconv.Itoa(i+1))
		}
	}

	return bad, nil
}

func isNonLowercaseKnownHeader(col string) bool {
	trimmed := strings.TrimSpace(col)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	if _, ok := checks.KnownHeaders[lower]; !ok {
		return false
	}

	return trimmed != lower
}
