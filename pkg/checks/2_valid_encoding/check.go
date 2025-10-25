package valid_encoding

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-utf8-encoding"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runUTF8Check,
		checks.WithFailFast(),
		checks.WithPriority(2),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runUTF8Check(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateUTF8,
		Fix:              fixUTF8,
		PassMsg:          "file encoding is valid UTF-8",
		FixedMsg:         "encoding fixed to valid UTF-8",
		AppliedMsg:       "auto-fix applied",
		StatusAfterFixed: checks.Pass,
	})
}

func validateUTF8(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	data := a.Data
	if len(data) == 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "empty file: cannot determine encoding (expected UTF-8)",
		}
	}

	if utf8.Valid(data) {
		return checks.ValidationResult{OK: true, Msg: "valid UTF-8"}
	}

	const checkEvery = 1 << 16
	for i := 0; i < len(data); {
		if (i & (checkEvery - 1)) == 0 {
			if err := ctx.Err(); err != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
			}
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return checks.ValidationResult{
				OK:  false,
				Msg: fmt.Sprintf("invalid UTF-8 sequence at byte %d of %d", i, len(data)),
			}
		}
		i += size
	}

	return checks.ValidationResult{OK: false, Msg: "invalid UTF-8 encoding"}
}
