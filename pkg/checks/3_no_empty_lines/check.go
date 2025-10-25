package empty_lines

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-no-empty-lines"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoEmptyLines,
		checks.WithPriority(3),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoEmptyLines(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateNoEmptyLines,
		Fix:              fixRemoveEmptyLines,
		PassMsg:          "no empty lines detected",
		FixedMsg:         "empty lines removed",
		AppliedMsg:       "auto-fix applied (blank lines removed)",
		StatusAfterFixed: checks.Pass,
		FailAs:           checks.Warn,
	})
}

func validateNoEmptyLines(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	total, first10, err := findEmptyLines(a.Data)
	if err != nil {
		return checks.ValidationResult{
			OK:  false,
			Msg: "failed to scan file for empty lines",
			Err: err,
		}
	}
	if total == 0 {
		return checks.ValidationResult{OK: true, Msg: ""}
	}
	return checks.ValidationResult{
		OK:  false,
		Msg: formatEmptyMsg(total, first10),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// scanning helpers
// ─────────────────────────────────────────────────────────────────────────────

// findEmptyLines scans with bufio.Scanner (large buffer) and returns the total
// number of empty (whitespace-only) lines and up to the first 10 line numbers.
func findEmptyLines(b []byte) (int, []int, error) {
	sc := bufio.NewScanner(bytes.NewReader(b))

	// allow long lines (16 MiB per line)
	const maxLine = 16 << 20
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)

	line, total := 0, 0
	first := make([]int, 0, 10)

	for sc.Scan() {
		line++
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			total++
			if len(first) < 10 {
				first = append(first, line)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return 0, nil, err
	}
	return total, first, nil
}

func formatEmptyMsg(total int, first []int) string {
	var sb strings.Builder
	if total == 1 {
		sb.WriteString("found 1 empty line")
	} else {
		sb.WriteString(fmt.Sprintf("found %d empty line(s)", total))
	}
	if len(first) > 0 {
		sb.WriteString(" at lines ")
		for i, n := range first {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(strconv.Itoa(n))
		}
		if more := total - len(first); more > 0 {
			sb.WriteString(fmt.Sprintf(" (+%d more)", more))
		}
	}
	return sb.String()
}
