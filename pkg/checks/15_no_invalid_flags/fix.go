package invalid_flags

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// fixNoInvalidFlags goes through the watched flag columns and tries to normalize
// values to "yes"/"no" if it's obviously mappable.
// Rules:
//   - trim spaces, lowercase
//   - "yes", "y", "true", "1"  -> "yes"
//   - "no", "n", "false", "0"  -> "no"
//   - "" (empty after trim)    -> leave as is (not auto-fixed)
//   - anything else            -> leave as is
//
// If at least one cell changed, we return DidChange=true and new data.
// If nothing changed, we return Errchecks.NoFix.
// splitRowCellsRaw keeps values verbatim (no TrimSpace). Use this in fix so we don't
// accidentally strip spaces from user content in non-flag columns.
func splitRowCellsRaw(s string) []string {
	return strings.Split(s, ";")
}

// fixNoInvalidFlags tries to normalize obvious variants of yes/no in watchedCols only.
// We DO NOT touch other cells except those columns, and we DO NOT trim other cells.
//
// Rules:
//   - take that cell's raw string, make lc-trimmed copy
//   - "yes","y","true","1"  => "yes"
//   - "no","n","false","0"  => "no"
//   - otherwise unchanged
//
// If nothing changes -> ErrNoFix so pipeline still FAILs.
func fixNoInvalidFlags(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	raw := string(a.Data)
	if raw == "" {
		return checks.NoFix(a, "no usable content to fix")
	}

	lines := strings.Split(raw, "\n")
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.NoFix(a, "no header line found")
	}

	headerLine := lines[headerIdx]
	if strings.TrimSpace(headerLine) == "" {
		return checks.NoFix(a, "empty header line")
	}

	headerCells := checks.SplitHeaderCells(headerLine)

	// map watchedCol -> header index
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

	// fast exit: if we don't have ANY watched column actually present
	allMissing := true
	for _, w := range watchedCols {
		if colPos[w] >= 0 {
			allMissing = false
			break
		}
	}
	if allMissing {
		return checks.NoFix(a, "no flag columns to normalize")
	}

	changed := false
	newLines := make([]string, 0, len(lines))

	// copy pre-header lines verbatim
	for i := 0; i < headerIdx; i++ {
		newLines = append(newLines, lines[i])
	}

	// copy header line verbatim
	newLines = append(newLines, headerLine)

	// process rows
	for rowIdx := headerIdx + 1; rowIdx < len(lines); rowIdx++ {
		rowRaw := lines[rowIdx]

		// keep empty/whitespace rows verbatim
		if strings.TrimSpace(rowRaw) == "" {
			newLines = append(newLines, rowRaw)
			continue
		}

		cells := splitRowCellsRaw(rowRaw)

		for _, w := range watchedCols {
			pos := colPos[w]
			if pos < 0 || pos >= len(cells) {
				continue
			}

			origVal := cells[pos]
			normVal := normalizeFlagValue(origVal)
			if normVal != origVal {
				cells[pos] = normVal
				changed = true
			}
		}

		newLines = append(newLines, strings.Join(cells, ";"))
	}

	if !changed {
		return checks.NoFix(a, "no flag values to normalize")
	}

	out := strings.Join(newLines, "\n")

	return checks.FixResult{
		Data:      []byte(out),
		Path:      "",
		DidChange: true,
		Note:      "normalized flag columns to yes/no",
	}, nil
}

// normalizeFlagValue maps obvious yes/no synonyms to canonical yes/no.
// DOES trim for the purpose of deciding, but returns canonical without extra spaces.
// If it can't decide, returns original untouched.
func normalizeFlagValue(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return v // keep empty; we won't auto-create yes/no from "nothing"
	}

	lc := strings.ToLower(trimmed)
	switch lc {
	case "yes", "y", "true", "1":
		return "yes"
	case "no", "n", "false", "0":
		return "no"
	default:
		return v
	}
}
