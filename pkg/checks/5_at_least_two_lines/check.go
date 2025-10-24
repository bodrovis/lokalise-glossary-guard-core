package at_least_two_lines

import (
	"context"
	"strings"

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
		Name:       checkName,
		Validate:   validateAtLeastTwoLines,
		PassMsg:    "file has at least two lines (header + data)",
		FailAs:     checks.Fail,
		FixedMsg:   "",
		AppliedMsg: "",
		Fix:        nil, // no fix available
	})
}

// validateAtLeastTwoLines checks if the file contains at least two non-empty lines.
// validateAtLeastTwoLines checks if the file contains at least two non-empty lines.
func validateAtLeastTwoLines(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	data := strings.TrimSpace(string(a.Data))
	if data == "" {
		return checks.ValidationResult{
			OK:  false,
			Msg: "empty file: expected header and at least one data row",
		}
	}

	lines := 0
	start := 0
	for start < len(data) {
		// find next newline
		idx := strings.IndexByte(data[start:], '\n')
		var line string
		if idx == -1 {
			line = data[start:]
			start = len(data)
		} else {
			line = data[start : start+idx]
			start += idx + 1
		}
		if strings.TrimSpace(line) != "" {
			lines++
			if lines >= 2 {
				break
			}
		}
	}

	if lines < 2 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "file must contain at least header and one data line",
		}
	}

	return checks.ValidationResult{OK: true}
}
