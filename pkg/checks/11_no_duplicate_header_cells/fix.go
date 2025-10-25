package duplicate_header_cells

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// fixDuplicateHeaderCells removes duplicate columns (2nd+ occurrences) from header and all data rows.
// Keeps first occurrence of each normalized header cell. Also drops duplicate empty headers "".
func fixDuplicateHeaderCells(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
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

	// header cells are assumed already trimmed by previous steps in the pipeline
	headerLine := lines[headerIdx]
	origCols := strings.Split(headerLine, ";")

	// decide which column indices to keep
	// first time we see normalized name -> keep
	// second+ time -> drop
	seen := make(map[string]bool)
	keepIdx := make([]int, 0, len(origCols))

	// we also build a list of removed names for Note
	removedNames := []string{}

	for i, col := range origCols {
		name := strings.TrimSpace(col)
		lc := strings.ToLower(name)

		if _, ok := seen[lc]; ok {
			// duplicate -> skip, and remember what we removed
			removedNames = append(removedNames, name)
			continue
		}
		seen[lc] = true
		keepIdx = append(keepIdx, i)
	}

	// if we didn't actually remove anything, report no-fix
	if len(removedNames) == 0 {
		return checks.NoFix(a, "no duplicate header columns to remove")
	}

	// rebuild header with only kept columns
	newHeaderCells := make([]string, 0, len(keepIdx))
	for _, idx := range keepIdx {
		if idx < len(origCols) {
			newHeaderCells = append(newHeaderCells, strings.TrimSpace(origCols[idx]))
		} else {
			newHeaderCells = append(newHeaderCells, "")
		}
	}
	lines[headerIdx] = strings.Join(newHeaderCells, ";")

	// rebuild every data row after headerIdx using the same keepIdx projection
	for row := headerIdx + 1; row < len(lines); row++ {
		rowRaw := lines[row]
		// leave fully empty / whitespace-only lines as-is
		if strings.TrimSpace(rowRaw) == "" {
			continue
		}

		values := strings.Split(rowRaw, ";")
		newValues := make([]string, 0, len(keepIdx))

		for _, idx := range keepIdx {
			if idx < len(values) {
				newValues = append(newValues, values[idx])
			} else {
				newValues = append(newValues, "")
			}
		}

		lines[row] = strings.Join(newValues, ";")
	}

	out := strings.Join(lines, "\n")

	note := "removed duplicate header columns: " + strings.Join(removedNames, ", ")

	return checks.FixResult{
		Data:      []byte(out),
		Path:      "",
		DidChange: true,
		Note:      note,
	}, nil
}
