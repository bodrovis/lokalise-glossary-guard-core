package empty_lines

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-no-empty-lines"

const (
	maxScannedLineSize = 16 << 20
	ctxCheckEveryLine  = 1 << 16
	maxReportedLines   = 10
)

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
		return cancelledValidation(err)
	}

	report, err := scanEmptyLines(ctx, a.Data)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return cancelledValidation(err)
		}

		return checks.ValidationResult{
			OK:  false,
			Msg: "failed to scan file for empty lines",
			Err: err,
		}
	}

	if !report.hasEmptyLines() {
		return checks.ValidationResult{OK: true, Msg: ""}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: report.format(),
	}
}

func (r emptyLinesReport) hasEmptyLines() bool {
	return r.total > 0
}

func (r emptyLinesReport) format() string {
	return formatEmptyMsg(r.total, r.first)
}

func (r *emptyLinesReport) add(lineNo int) {
	r.total++

	if len(r.first) < maxReportedLines {
		r.first = append(r.first, lineNo)
	}
}

func checkContextEveryLine(ctx context.Context, lineNo int) error {
	if lineNo%ctxCheckEveryLine != 0 {
		return nil
	}

	return ctx.Err()
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}

func formatEmptyMsg(total int, first []int) string {
	var sb strings.Builder

	if total == 1 {
		sb.WriteString("found 1 empty line")
	} else {
		fmt.Fprintf(&sb, "found %d empty lines", total)
	}

	if len(first) == 0 {
		return sb.String()
	}

	sb.WriteString(" at lines ")
	sb.WriteString(formatLineNumbers(first))

	if more := total - len(first); more > 0 {
		fmt.Fprintf(&sb, " (+%d more)", more)
	}

	return sb.String()
}

func formatLineNumbers(lines []int) string {
	var sb strings.Builder

	for i, line := range lines {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Itoa(line))
	}

	return sb.String()
}
