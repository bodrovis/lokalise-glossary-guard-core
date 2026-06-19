package no_spaces_in_header

import (
	"context"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// Ensures header column names have no leading/trailing spaces.
// Internal spaces inside names are not checked here.
const checkName = "no-spaces-in-header"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoSpacesInHeader,
		checks.WithFailFast(),
		checks.WithPriority(7),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoSpacesInHeader(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateNoSpacesInHeader,
		Fix:              fixNoSpacesInHeader,
		FailAs:           checks.Warn,
		PassMsg:          "header columns are trimmed (no leading/trailing spaces)",
		FixedMsg:         "header auto-fixed: trimmed leading/trailing spaces in column names",
		AppliedMsg:       "auto-fix applied to header",
		StillBadMsg:      "auto-fix attempted but header still invalid",
		StatusAfterFixed: checks.Pass,
	})
}

func validateNoSpacesInHeader(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	record, res, ok := readHeaderRecord(ctx, a)
	if !ok {
		return res
	}

	badCols, err := findHeaderColumnsWithSpaces(ctx, record)
	if err != nil {
		return cancelledValidation(err)
	}

	if len(badCols) > 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "header has leading/trailing spaces in column names at positions: " + strings.Join(badCols, ", "),
		}
	}

	return checks.ValidationResult{
		OK:  true,
		Msg: "header columns are trimmed (no leading/trailing spaces)",
	}
}

func readHeaderRecord(ctx context.Context, a checks.Artifact) ([]string, checks.ValidationResult, bool) {
	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		ctx,
		a,
		"cannot check header: empty content",
	)
	if !ok {
		return nil, res, false
	}

	record, err := r.Read()
	if err != nil || len(record) == 0 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, cancelledValidation(ctxErr), false
		}

		return nil, checks.ValidationResult{
			OK:  false,
			Msg: "cannot parse header with semicolon delimiter",
			Err: err,
		}, false
	}

	return record, checks.ValidationResult{}, true
}

func findHeaderColumnsWithSpaces(ctx context.Context, record []string) ([]string, error) {
	var badCols []string

	for i, col := range record {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if hasOuterSpace(col) {
			badCols = append(badCols, strconv.Itoa(i+1))
		}
	}

	return badCols, nil
}

func hasOuterSpace(s string) bool {
	return s != strings.TrimSpace(s)
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
