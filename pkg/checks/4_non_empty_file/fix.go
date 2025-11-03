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

func fixAddHeaderIfEmpty(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	data := a.Data
	// strip optional UTF-8 BOM
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		data = data[3:]
	}
	if len(bytes.TrimSpace(data)) != 0 {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "file already has data; no header inserted"}, nil
	}

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

	header := strings.Join(fields, ";")
	newData := []byte(header)

	return checks.FixResult{
		Data:      newData,
		Path:      "",
		DidChange: true,
		Note:      "inserted CSV header",
	}, nil
}
