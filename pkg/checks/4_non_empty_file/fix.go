package non_empty_file

import (
	"bytes"
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

var baseHeaderFields = []string{
	"term",
	"description",
	"casesensitive",
	"translatable",
	"forbidden",
	"tags",
}

// fixAddHeaderIfEmpty inserts a minimal CSV header when the file is effectively empty.
// "Empty" means the content contains only whitespace and/or zero-width/invisible runes.
// It does not append a trailing line ending. If the file already has content, it is left unchanged.
func fixAddHeaderIfEmpty(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	data := a.Data

	// Optionally strip UTF-8 BOM at the start; we don't keep it in the output header.
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		data = data[3:]
	}

	// Consider the file empty if it contains only whitespace/zero-width.
	if !checks.IsBlankUnicode(bytes.TrimSpace(data)) && !checks.IsBlankUnicode(data) {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "file already has data; no header inserted",
		}, nil
	}

	// Build header: base fields + per-language fields (lowercased, deduped).
	seen := make(map[string]struct{}, len(a.Langs))
	fields := make([]string, 0, len(baseHeaderFields)+len(a.Langs)*2)
	fields = append(fields, baseHeaderFields...)

	for _, lang := range a.Langs {
		lc := strings.ToLower(strings.TrimSpace(lang))
		if lc == "" {
			continue
		}
		if _, ok := seen[lc]; ok {
			continue
		}
		seen[lc] = struct{}{}
		fields = append(fields, lc, lc+"_description")
	}

	// Use ';' as a delimiter to match downstream parsing.
	header := strings.Join(fields, ";")

	return checks.FixResult{
		Data:      []byte(header), // no trailing newline by design
		Path:      "",
		DidChange: true,
		Note:      "inserted CSV header",
	}, nil
}
