package lowercase_header

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixLowercaseHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	raw := string(a.Data)
	if raw == "" {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no usable content to normalize header",
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
		originalCell := c

		trimmed := strings.TrimSpace(c)
		lower := strings.ToLower(trimmed)

		if _, mustLower := requiredLowercaseCols[lower]; mustLower {
			if originalCell != lower {
				changed = true
			}
			cells[i] = lower
			continue
		}

		cells[i] = originalCell
	}

	if !changed {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "header service columns already lowercase",
		}, nil
	}

	newHeader := strings.Join(cells, ";")
	lines[headerIdx] = newHeader
	out := strings.Join(lines, "\n")

	return checks.FixResult{
		Data:      []byte(out),
		Path:      "",
		DidChange: true,
		Note:      "normalized service columns in header to lowercase",
	}, nil
}
