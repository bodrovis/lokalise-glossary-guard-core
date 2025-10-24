package non_empty_file

import (
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

	if len(strings.TrimSpace(string(a.Data))) != 0 {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "file already has data; no header inserted",
		}, nil
	}

	fields := make([]string, 0, len(baseHeaderFields)+len(a.Langs)*2)
	fields = append(fields, baseHeaderFields...)
	for _, lang := range a.Langs {
		lc := strings.ToLower(strings.TrimSpace(lang))
		if lc == "" {
			continue
		}
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
