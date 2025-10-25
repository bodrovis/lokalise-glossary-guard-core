package term_description_header

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-term-description-header"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureTermDescriptionHeader,
		checks.WithFailFast(),
		checks.WithPriority(9),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runEnsureTermDescriptionHeader(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateTermDescriptionHeader,
		Fix:              fixTermDescriptionHeader,
		PassMsg:          "header starts with term;description",
		FixedMsg:         "normalized header to start with term;description",
		AppliedMsg:       "auto-fix applied: normalized header to start with term;description",
		StatusAfterFixed: checks.Pass,
		StillBadMsg:      "header still does not start with term;description after fix",
	})
}

func validateTermDescriptionHeader(ctx context.Context, a checks.Artifact) checks.ValidationResult {
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

	lines := strings.Split(raw, "\n")
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot check header: no usable content",
		}
	}

	header := lines[headerIdx]
	cells := strings.Split(header, ";")
	if len(cells) < 2 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "header has fewer than two columns; expected at least term;description",
		}
	}

	// check first two cells (lowercase already guaranteed by previous check)
	first := strings.TrimSpace(cells[0])
	second := strings.TrimSpace(cells[1])

	if first == "term" && second == "description" {
		return checks.ValidationResult{
			OK:  true,
			Msg: "header starts with term;description",
		}
	}

	// ok, figure out what went wrong for more useful message
	hasTerm := false
	hasDesc := false
	for _, c := range cells {
		switch cc := strings.TrimSpace(c); cc {
		case "term":
			hasTerm = true
		case "description":
			hasDesc = true
		}
	}

	switch {
	case hasTerm && hasDesc:
		return checks.ValidationResult{
			OK:  false,
			Msg: "header contains term and description but not in required order or not at the start",
		}
	case hasTerm && !hasDesc:
		return checks.ValidationResult{
			OK:  false,
			Msg: "header contains term but missing description column",
		}
	case !hasTerm && hasDesc:
		return checks.ValidationResult{
			OK:  false,
			Msg: "header contains description but missing term column",
		}
	default:
		return checks.ValidationResult{
			OK:  false,
			Msg: "header missing both term and description columns",
		}
	}
}

// same helper as before
