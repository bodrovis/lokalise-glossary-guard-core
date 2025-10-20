package valid_extension

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// fixCSVExt renames the artifact path to have a ".csv" extension (lowercase).
// Data is not modified. If context is canceled, returns an error.
func fixCSVExt(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	fp := strings.TrimSpace(a.Path)
	if fp == "" {
		return checks.FixResult{
			Data:      a.Data,
			Path:      a.Path,
			DidChange: false,
			Note:      "empty path: nothing to fix",
		}, nil
	}

	ext := filepath.Ext(fp)
	base := strings.TrimSuffix(fp, ext)
	// avoid "name..csv" when original had a trailing dot like "name."
	base = strings.TrimRight(base, ".")

	newPath := base + ".csv"
	changed := newPath != a.Path

	note := "already has .csv extension"
	if changed {
		note = "renamed to .csv"
	}

	return checks.FixResult{
		Data:      a.Data,
		Path:      newPath,
		DidChange: changed,
		Note:      note,
	}, nil
}
