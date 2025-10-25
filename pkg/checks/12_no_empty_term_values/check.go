package no_empty_term_values

import (
	"context"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-empty-term-values"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoEmptyTermValues,
		checks.WithFailFast(),
		checks.WithPriority(12),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoEmptyTermValues(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:     checkName,
		Validate: validateNoEmptyTermValues,
		Fix:      nil,
		PassMsg:  "all rows have non-empty term",
	})
}

func validateNoEmptyTermValues(ctx context.Context, a checks.Artifact) checks.ValidationResult {
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
			OK:  true,
			Msg: "no content to validate for empty term values",
		}
	}

	lines := strings.Split(raw, "\n")
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no header line found (nothing to validate for empty term values)",
		}
	}

	headerLine := lines[headerIdx]
	if strings.TrimSpace(headerLine) == "" {
		return checks.ValidationResult{
			OK:  true,
			Msg: "empty header line (nothing to validate for empty term values)",
		}
	}

	headerCells := splitCells(headerLine)

	termCol := -1
	for i, h := range headerCells {
		lc := strings.ToLower(strings.TrimSpace(h))
		if lc == "term" {
			termCol = i
			break
		}
	}

	if termCol < 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no 'term' column found (skipping empty term validation)",
		}
	}

	var badRows []int

	for rowIdx := headerIdx + 1; rowIdx < len(lines); rowIdx++ {
		rawRow := lines[rowIdx]

		if strings.TrimSpace(rawRow) == "" {
			continue
		}

		cells := splitCells(rawRow)

		val := ""
		if termCol < len(cells) {
			val = strings.TrimSpace(cells[termCol])
		}

		if val == "" {
			badRows = append(badRows, rowIdx+1)
		}
	}

	if len(badRows) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "all rows have non-empty term",
		}
	}

	displayRows := badRows
	if len(displayRows) > 10 {
		displayRows = displayRows[:10]
	}

	msg := "empty term in rows: " + joinIntSlice(displayRows, ", ")
	if len(badRows) > 10 {
		msg += " ... (total " + strconv.Itoa(len(badRows)) + ")"
	} else {
		msg += " (total " + strconv.Itoa(len(badRows)) + ")"
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: msg,
	}
}

func splitCells(s string) []string {
	parts := strings.Split(s, ";")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func joinIntSlice(nums []int, sep string) string {
	if len(nums) == 0 {
		return ""
	}
	if len(nums) == 1 {
		return strconv.Itoa(nums[0])
	}
	var b strings.Builder
	b.WriteString(strconv.Itoa(nums[0]))
	for i := 1; i < len(nums); i++ {
		b.WriteString(sep)
		b.WriteString(" ")
		b.WriteString(strconv.Itoa(nums[i]))
	}
	return b.String()
}
