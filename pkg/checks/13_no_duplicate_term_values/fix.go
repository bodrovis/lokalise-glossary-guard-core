package duplicate_term_values

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// fixDuplicateTermValues removes rows that repeat an already-seen term value.
// Case-sensitive on the term values themselves.
// We keep pre-header "preamble" lines untouched. We keep the header line.
// Then we keep the first row for each distinct term, drop subsequent ones.
// Blank/whitespace-only rows are preserved. Empty term values are preserved
// (check 12 handles them).
func fixDuplicateTermValues(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
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
		return checks.NoFix(a, "no header with 'term' column found")
	}

	headerLine := lines[headerIdx]
	headerCells := splitCells(headerLine)

	termCol := -1
	for i, h := range headerCells {
		if strings.ToLower(strings.TrimSpace(h)) == "term" {
			termCol = i
			break
		}
	}
	if termCol < 0 {
		return checks.NoFix(a, "no 'term' column found")
	}

	seen := make(map[string]bool)

	type removedInfo struct {
		term string
		rows []int
	}
	removedByTerm := make(map[string]*removedInfo)

	outLines := make([]string, 0, len(lines))

	// 1. copy all lines BEFORE headerIdx as-is (preamble)
	for i := 0; i < headerIdx; i++ {
		outLines = append(outLines, lines[i])
	}

	// 2. copy header line itself
	outLines = append(outLines, headerLine)

	// 3. process rows AFTER headerIdx
	for rowIdx := headerIdx + 1; rowIdx < len(lines); rowIdx++ {
		rowRaw := lines[rowIdx]

		// preserve raw blank lines
		if strings.TrimSpace(rowRaw) == "" {
			outLines = append(outLines, rowRaw)
			continue
		}

		cells := splitCells(rowRaw)

		val := ""
		if termCol < len(cells) {
			val = strings.TrimSpace(cells[termCol])
		}

		// empty term? keep (check 12 will scream elsewhere)
		if val == "" {
			outLines = append(outLines, rowRaw)
			continue
		}

		if !seen[val] {
			seen[val] = true
			outLines = append(outLines, rowRaw)
			continue
		}

		// duplicate -> drop this row
		info := removedByTerm[val]
		if info == nil {
			info = &removedInfo{
				term: val,
			}
			removedByTerm[val] = info
		}
		info.rows = append(info.rows, rowIdx+1) // human-readable row number
	}

	if len(removedByTerm) == 0 {
		return checks.NoFix(a, "no duplicate term rows to remove")
	}

	// build note
	var noteBuilder strings.Builder
	noteBuilder.WriteString("removed duplicate term rows for: ")
	i := 0
	for _, info := range removedByTerm {
		if i > 0 {
			noteBuilder.WriteString("; ")
		}
		noteBuilder.WriteString(`"`)
		noteBuilder.WriteString(info.term)
		noteBuilder.WriteString(`" (rows `)
		noteBuilder.WriteString(joinIntSlice(info.rows, ", "))
		noteBuilder.WriteString(`)`)
		i++
	}

	out := strings.Join(outLines, "\n")

	return checks.FixResult{
		Data:      []byte(out),
		Path:      "",
		DidChange: true,
		Note:      noteBuilder.String(),
	}, nil
}
