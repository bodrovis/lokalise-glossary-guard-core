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
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
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
		normalized := strings.ToLower(strings.TrimSpace(c))
		if _, mustLower := requiredLowercaseCols[normalized]; mustLower {
			if c != normalized {
				changed = true
			}
			cells[i] = normalized
		} else {
			cells[i] = c
		}
	}

	if !changed {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "header service columns already lowercase",
		}, nil
	}

	lines[headerIdx] = strings.Join(cells, ";")
	out := strings.Join(lines, "\n")

	return checks.FixResult{
		Data:      []byte(out),
		Path:      "",
		DidChange: true,
		Note:      "normalized service columns in header to lowercase",
	}, nil
}
