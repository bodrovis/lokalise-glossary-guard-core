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

	data := checks.StripUTF8BOM(a.Data)

	if !isEffectivelyEmpty(data) {
		return noHeaderInserted(a), nil
	}

	header := buildCSVHeader(a.Langs)

	return checks.FixResult{
		Data:      []byte(header),
		Path:      "",
		DidChange: true,
		Note:      "inserted CSV header",
	}, nil
}

func isEffectivelyEmpty(data []byte) bool {
	// Keep the old behavior:
	// - raw blank-looking content counts as empty
	// - TrimSpace-normalized blank-looking content also counts as empty
	return checks.IsBlankUnicode(data) || checks.IsBlankUnicode(bytes.TrimSpace(data))
}

func noHeaderInserted(a checks.Artifact) checks.FixResult {
	return checks.FixResult{
		Data:      a.Data,
		Path:      "",
		DidChange: false,
		Note:      "file already has data; no header inserted",
	}
}

func buildCSVHeader(langs []string) string {
	return strings.Join(buildHeaderFields(langs), ";")
}

func buildHeaderFields(langs []string) []string {
	fields := make([]string, 0, len(baseHeaderFields)+len(langs)*2)
	fields = append(fields, baseHeaderFields...)

	appendLanguageFields(&fields, langs)

	return fields
}

func appendLanguageFields(fields *[]string, langs []string) {
	seen := make(map[string]struct{}, len(langs))

	for _, lang := range langs {
		lc := normalizeLang(lang)
		if lc == "" {
			continue
		}

		if _, ok := seen[lc]; ok {
			continue
		}
		seen[lc] = struct{}{}

		*fields = append(*fields, lc, lc+"_description")
	}
}

func normalizeLang(lang string) string {
	return strings.ToLower(strings.TrimSpace(lang))
}
