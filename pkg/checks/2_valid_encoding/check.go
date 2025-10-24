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

// runUTF8Check wires the recipe: validate → maybe fix → maybe revalidate.
// NOTE: FailAs omitted → defaults to FAIL (works well with FailFast).
func runUTF8Check(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateUTF8,
		Fix:              fixUTF8, // will implement in fix.go
		PassMsg:          "file encoding is valid UTF-8",
		FixedMsg:         "encoding fixed to valid UTF-8",
		AppliedMsg:       "auto-fix applied",
		StatusAfterFixed: checks.Pass, // trust the recode → PASS
	})
}

// validateUTF8 inspects bytes and reports first invalid position (if any).
// It’s panic-safe via RunWithFix’s safeValidate wrapper.
func validateUTF8(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	data := a.Data
	if len(data) == 0 {
		// policy choice: empty file = cannot determine → FAIL (not ERROR)
		return checks.ValidationResult{OK: false, Msg: "empty file: cannot determine encoding"}
	}

	// Fast path: entirely valid
	if utf8.Valid(data) {
		return checks.ValidationResult{OK: true}
	}

	// Find the first offending byte for a nicer message.
	const checkEvery = 1 << 16 // periodically check ctx for large blobs
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
				Msg: fmt.Sprintf("invalid UTF-8 at byte %d of %d", i, len(data)),
			}
		}
		i += size
	}

	// If we got here, we saw RuneError with size>1 somewhere (rare), or mixed state; still invalid.
	return checks.ValidationResult{OK: false, Msg: "invalid UTF-8"}
}
