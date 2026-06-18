package valid_encoding

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const (
	checkName      = "ensure-utf8-encoding"
	checkEveryByte = 1 << 16
)

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
		return cancelledValidation(err)
	}

	if res, ok := validateNonEmptyData(a.Data); !ok {
		return res
	}

	pos, err := firstInvalidUTF8Byte(ctx, a.Data)
	if err != nil {
		return cancelledValidation(err)
	}
	if pos >= 0 {
		return checks.ValidationResult{
			OK: false,
			Msg: fmt.Sprintf(
				"invalid UTF-8 sequence at byte %d of %d",
				pos,
				len(a.Data),
			),
		}
	}

	return checks.ValidationResult{OK: true, Msg: "valid UTF-8"}
}

func validateNonEmptyData(data []byte) (checks.ValidationResult, bool) {
	if len(data) > 0 {
		return checks.ValidationResult{}, true
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: "empty file: cannot determine encoding (expected UTF-8)",
	}, false
}

func firstInvalidUTF8Byte(ctx context.Context, data []byte) (int, error) {
	for i := 0; i < len(data); {
		if i%checkEveryByte == 0 {
			if err := ctx.Err(); err != nil {
				return -1, err
			}
		}

		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return i, nil
		}

		i += size
	}

	return -1, nil
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
