package invalid_flags

import (
	"context"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-invalid-flags"

// columns we treat as boolean flags
var watchedCols = []string{
	"casesensitive",
	"translatable",
	"forbidden",
}

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoInvalidFlags,
		checks.WithFailFast(),   // hard fail if bad values appear
		checks.WithPriority(15), // runs after all structural cleanup
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoInvalidFlags(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateNoInvalidFlags,
		Fix:              fixNoInvalidFlags,
		PassMsg:          "all flag columns contain only yes/no",
		FixedMsg:         "normalized flag columns to yes/no",
		AppliedMsg:       "auto-fix applied: normalized flag columns to yes/no",
		StatusAfterFixed: checks.Pass,
		// FailAs defaults to FAIL (hard fail)
		StillBadMsg: "invalid flag values remain after fix",
	})
}

// validateNoInvalidFlags checks that all watchedCols (if present in header)
// only contain "yes" or "no" (case-sensitive) and not empty.
func validateNoInvalidFlags(ctx context.Context, a checks.Artifact) checks.ValidationResult {
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
			Msg: "no content to validate for flags",
		}
	}

	lines := strings.Split(raw, "\n")
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no header line found (nothing to validate for flags)",
		}
	}

	headerLine := lines[headerIdx]
	if strings.TrimSpace(headerLine) == "" {
		return checks.ValidationResult{
			OK:  true,
			Msg: "empty header line (nothing to validate for flags)",
		}
	}

	headerCells := checks.SplitHeaderCells(headerLine)

	// map watchedCol -> index in header (or -1 if absent)
	colPos := make(map[string]int, len(watchedCols))
	for _, w := range watchedCols {
		colPos[w] = -1
	}
	for idx, h := range headerCells {
		lc := strings.ToLower(strings.TrimSpace(h))
		if _, ok := colPos[lc]; ok {
			colPos[lc] = idx
		}
	}

	type bad struct {
		colName string
		val     string
		rowNum  int // 1-based
	}
	var invalids []bad

	for rowIdx := headerIdx + 1; rowIdx < len(lines); rowIdx++ {
		rawRow := lines[rowIdx]

		if strings.TrimSpace(rawRow) == "" {
			continue
		}

		// NOTE: for validation мы можем триммить ячейки, это не мутирует файл
		cells := checks.SplitHeaderCells(rawRow)

		for _, w := range watchedCols {
			pos := colPos[w]
			if pos < 0 {
				continue // this column doesn't exist
			}

			val := ""
			if pos < len(cells) {
				val = strings.TrimSpace(cells[pos])
			}

			// must be exactly "yes" or "no", can't be empty
			if val != "yes" && val != "no" {
				invalids = append(invalids, bad{
					colName: w,
					val:     val,
					rowNum:  rowIdx + 1,
				})
			}
		}
	}

	if len(invalids) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "all flag columns contain only yes/no",
		}
	}

	limit := 10
	if len(invalids) < limit {
		limit = len(invalids)
	}

	var b strings.Builder
	b.WriteString("invalid values in flag columns: ")
	for i := 0; i < limit; i++ {
		inv := invalids[i]
		// ex: casesensitive="maybe" (row 5)
		b.WriteString(inv.colName)
		b.WriteString(`="`)
		b.WriteString(inv.val)
		b.WriteString(`" (row `)
		b.WriteString(strconv.Itoa(inv.rowNum))
		b.WriteString(`)`)
		if i != limit-1 {
			b.WriteString("; ")
		}
	}

	if len(invalids) > limit {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(invalids)))
		b.WriteString(" invalid values)")
	} else {
		b.WriteString(" (total ")
		b.WriteString(strconv.Itoa(len(invalids)))
		b.WriteString(" invalid values)")
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: b.String(),
	}
}
