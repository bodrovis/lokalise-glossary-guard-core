package valid_encoding

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-utf8-encoding"

var fixer checks.FixFunc = fixUTF8

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runUTF8Check,
		checks.WithFailFast(),
		checks.WithPriority(2),
		checks.WithRecover(),
	)
	if err != nil {
		panic("ensure_utf8: " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic("ensure_utf8 register: " + err.Error())
	}
}

// Run orchestrates validation → optional fix → optional re-validation.
func runUTF8Check(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateUTF8,
		Fix:              fixer,
		PassMsg:          "file encoding is valid UTF-8",
		FixedMsg:         "encoding fixed to valid UTF-8",
		AppliedMsg:       "auto-fix applied",
		StatusAfterFixed: checks.Pass,
	})
}

// validateUTF8 does only validation, no side effects.
func validateUTF8(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{
			OK:  false,
			Msg: "validation cancelled",
			Err: err,
		}
	}

	data := a.Data
	if len(data) == 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "empty file: cannot determine encoding",
			Err: nil,
		}
	}

	if utf8.Valid(data) {
		return checks.ValidationResult{
			OK:  true,
			Msg: "",
			Err: nil,
		}
	}

	const checkEvery = 1 << 16
	i := 0
	for i < len(data) {
		if (i & (checkEvery - 1)) == 0 {
			if err := ctx.Err(); err != nil {
				return checks.ValidationResult{
					OK:  false,
					Msg: "validation cancelled",
					Err: err,
				}
			}
		}
		r, size := utf8.DecodeRune(data[i:])

		if r == utf8.RuneError && size == 1 {
			return checks.ValidationResult{
				OK:  false,
				Msg: fmt.Sprintf("invalid UTF-8 at byte %d of %d", i, len(data)),
				Err: nil,
			}
		}
		i += size
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: "invalid UTF-8",
		Err: nil,
	}
}
