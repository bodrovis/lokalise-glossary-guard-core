package lowercase_header

import (
	"context"
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

// policy:
// - FailAs: WARN → не стопаем пайплайн
// - StatusAfterFixed: PASS → если автофикс прошёл и ревалиднули ок, считаем PASS
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
		return checks.ValidationResult{
			OK:  false,
			Msg: "validation cancelled",
			Err: err,
		}
	}

	raw := string(a.Data)
	if raw == "" {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot check header: no usable content",
		}
	}

	lines := splitLinesPreserveAll(raw)
	headerLineIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerLineIdx < 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot check header: no usable content",
		}
	}

	header := lines[headerLineIdx]

	start := 0
	for i := 0; i <= len(header); i++ {
		if i == len(header) || header[i] == ';' {
			cell := header[start:i]
			start = i + 1

			if err := ctx.Err(); err != nil {
				return checks.ValidationResult{
					OK:  false,
					Msg: "validation cancelled",
					Err: err,
				}
			}

			trimmed := strings.TrimSpace(cell)
			if trimmed == "" {
				continue
			}

			normalized := strings.ToLower(trimmed)

			if _, isRequired := requiredLowercaseCols[normalized]; !isRequired {
				continue
			}

			if trimmed != normalized {
				return checks.ValidationResult{
					OK:  false,
					Msg: "some service columns in header are not lowercase (expected: term;description;casesensitive;translatable;forbidden;tags)",
				}
			}
		}
	}

	return checks.ValidationResult{
		OK:  true,
		Msg: "header service columns are already lowercase",
	}
}

// helper: split into lines without dropping anything.
func splitLinesPreserveAll(s string) []string {
	return strings.Split(s, "\n")
}
