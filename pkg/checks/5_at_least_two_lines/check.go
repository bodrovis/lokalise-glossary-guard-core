package at_least_two_lines

import (
	"bufio"
	"bytes"
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-at-least-two-lines"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureAtLeastTwoLines,
		checks.WithFailFast(),
		checks.WithPriority(5),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

// runEnsureAtLeastTwoLines — entry point for the check.
// There is no auto-fix for this one.
func runEnsureAtLeastTwoLines(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:     checkName,
		Validate: validateAtLeastTwoLines,
		PassMsg:  "file has at least two lines (header + data)",
		FailAs:   checks.Fail,
	})
}

func validateAtLeastTwoLines(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	data := a.Data

	// Strip optional UTF-8 BOM; do not let it make the file look "non-empty".
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		data = data[3:]
	}

	// If effectively empty (only whitespace/zero-width), fail early.
	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  false,
			Msg: "empty file: expected header and at least one data row",
		}
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	const maxLine = 16 << 20 // 16 MiB per line
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)

	lines := 0
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
		}
		chunk := sc.Bytes() // split by '\n'; a trailing '\r' may be present

		// Normalize CRLF by stripping trailing '\r' from the chunk.
		if n := len(chunk); n > 0 && chunk[n-1] == '\r' {
			chunk = chunk[:n-1]
		}

		// Skip blank-looking lines (Unicode whitespace + zero-widths).
		if checks.IsBlankUnicode(chunk) {
			continue
		}

		lines++
		if lines >= 2 {
			return checks.ValidationResult{OK: true, Msg: "has ≥2 lines"}
		}
	}
	if err := sc.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "failed to read file", Err: err}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: "expected at least two non-empty lines (header + one data row)",
	}
}
