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

func validateLowercaseHeader(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	header, res, ok := readHeader(ctx, a)
	if !ok {
		return res
	}

	bad, err := findNonLowercaseServiceHeaderColumns(ctx, header)
	if err != nil {
		return cancelledValidation(err)
	}

	if len(bad) > 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "some service columns in header are not lowercase at positions: " + strings.Join(bad, ", "),
		}
	}

	return checks.ValidationResult{
		OK:  true,
		Msg: "header service columns are already lowercase",
	}
}

func readHeader(ctx context.Context, a checks.Artifact) ([]string, checks.ValidationResult, bool) {
	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		ctx,
		a,
		"cannot check header: no usable content",
	)
	if !ok {
		return nil, res, false
	}

	header, err := r.Read()
	if err != nil || len(header) == 0 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, cancelledValidation(ctxErr), false
		}

		return nil, checks.ValidationResult{
			OK:  false,
			Msg: "cannot parse header with semicolon delimiter",
			Err: err,
		}, false
	}

	return header, checks.ValidationResult{}, true
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

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
