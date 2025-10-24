package no_spaces_in_header

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixNoSpacesInHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	raw := string(a.Data)
	if raw == "" {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no usable content to trim header",
		}, checks.ErrNoFix
	}

	lines := splitLinesPreserveAll(raw)
	headerIdx := firstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no header line found",
		}, checks.ErrNoFix
	}

	origHeader := lines[headerIdx]
	cells := strings.Split(origHeader, ";")

	changed := false
	for i, c := range cells {
		trimmed := strings.TrimSpace(c)
		if trimmed != c {
			changed = true
			cells[i] = trimmed
		}
	}

	if !changed {
		// уже всё чисто, фикс по сути не нужен
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "header already trimmed",
		}, nil
	}

	newHeader := strings.Join(cells, ";")
	lines[headerIdx] = newHeader

	out := strings.Join(lines, "\n")

	return checks.FixResult{
		Data:      []byte(out),
		Path:      "",
		DidChange: true,
		Note:      "trimmed leading/trailing spaces in header cells",
	}, nil
}

// helper: split into lines without dropping anything.
// basically strings.Split but keeps empty trailing last line predictable.
func splitLinesPreserveAll(s string) []string {
	return strings.Split(s, "\n")
}

func firstNonEmptyLineIndex(lines []string) int {
	for i, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			return i
		}
	}
	return -1
}
