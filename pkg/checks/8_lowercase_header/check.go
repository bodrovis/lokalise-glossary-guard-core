package lowercase_header

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-lowercase-header"

var requiredLowercaseCols = map[string]struct{}{
	"term":          {},
	"description":   {},
	"casesensitive": {},
	"translatable":  {},
	"forbidden":     {},
	"tags":          {},
}

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

// policy: FailAs WARN, StatusAfterFixed PASS — оставляем как есть
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
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: false, Msg: "cannot check header: no usable content"}
	}

	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil || len(header) == 0 {
		if ctx.Err() != nil {
			return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
		}
		return checks.ValidationResult{OK: false, Msg: "cannot parse header with semicolon delimiter", Err: err}
	}

	var bad []string
	for i, col := range header {
		if err := ctx.Err(); err != nil {
			return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
		}
		trimmed := strings.TrimSpace(col)
		if trimmed == "" {
			continue
		}
		lc := strings.ToLower(trimmed)
		if _, want := requiredLowercaseCols[lc]; !want {
			continue
		}
		if trimmed != lc {
			bad = append(bad, strconv.Itoa(i+1))
		}
	}

	if len(bad) > 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "some service columns in header are not lowercase at positions: " + strings.Join(bad, ", "),
		}
	}
	return checks.ValidationResult{OK: true, Msg: "header service columns are already lowercase"}
}
