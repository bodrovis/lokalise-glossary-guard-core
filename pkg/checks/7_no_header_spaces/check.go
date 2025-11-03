package no_spaces_in_header

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

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
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: false, Msg: "cannot check header: empty content"}
	}

	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	record, err := r.Read()
	if err != nil || len(record) == 0 {
		if ctx.Err() != nil {
			return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
		}
		return checks.ValidationResult{OK: false, Msg: "cannot parse header with semicolon delimiter", Err: err}
	}

	var badCols []string
	for i, col := range record {
		if err := ctx.Err(); err != nil {
			return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
		}
		if col != strings.TrimSpace(col) {
			badCols = append(badCols, strconv.Itoa(i+1)) // 1-based index
		}
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
