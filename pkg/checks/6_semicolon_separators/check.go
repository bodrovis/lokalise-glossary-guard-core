package semicolon_separator

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

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
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	dataBytes := a.Data
	if bytes.HasPrefix(dataBytes, []byte{0xEF, 0xBB, 0xBF}) {
		dataBytes = dataBytes[3:]
	}
	if checks.IsBlankUnicode(dataBytes) {
		return checks.ValidationResult{OK: false, Msg: "cannot detect separators: no usable content"}
	}
	data := string(a.Data)

	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	semiOK, _ := attemptRectParse(data, ';')
	if semiOK {
		return checks.ValidationResult{OK: true}
	}

	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	commaOK, _ := attemptRectParse(data, ',')
	tabOK, _ := attemptRectParse(data, '\t')

	switch {
	case commaOK:
		return checks.ValidationResult{
			OK:  false,
			Msg: "file appears to use commas as separators; expected semicolons (;)",
		}
	case tabOK:
		return checks.ValidationResult{
			OK:  false,
			Msg: "file appears to use tabs as separators; expected semicolons (;)",
		}
	default:
		return checks.ValidationResult{
			OK:  false,
			Msg: "could not confirm consistent semicolon-separated format; cannot confidently detect an alternative delimiter",
		}
	}
}

// attemptRectParse tries to parse data with the given delimiter using encoding/csv
// and then validates that the result is a proper "table":
// - at least one non-empty record
// - every record has the same number of fields
// - that number of fields > 1 (so we don't 'convert' single-column junk)
//
// returns (isRectangular, records)
func attemptRectParse(data string, delim rune) (bool, [][]string) {
	r := csv.NewReader(strings.NewReader(data))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	recs, err := r.ReadAll()
	if err != nil || len(recs) == 0 {
		return false, nil
	}

	var width int
	for _, row := range recs {
		if len(row) == 0 {
			continue
		}
		if len(row) > 1 || (len(row) == 1 && strings.TrimSpace(row[0]) != "") {
			width = len(row)
			break
		}
	}
	if width <= 1 {
		return false, nil
	}

	for _, row := range recs {
		if len(row) == 0 {
			continue
		}
		if len(row) != width {
			return false, nil
		}
	}
	return true, recs
}
