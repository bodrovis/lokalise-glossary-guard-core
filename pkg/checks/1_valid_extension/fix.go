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

	orig := a.Path
	fp := strings.TrimSpace(orig)
	if fp == "" {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "", // keep original (empty)
			DidChange: false,
			Note:      "empty path: nothing to fix",
		}, nil
	}

	ext := filepath.Ext(fp)              // ".txt" | ".CSV" | ""
	base := strings.TrimSuffix(fp, ext)  // "name" | "archive.tar"
	base = strings.TrimSuffix(base, ".") // "name." -> "name"

	newPath := base + ".csv"
	changed := newPath != fp

	note := "already has .csv extension"
	if changed {
		note = "renamed to .csv"
	}

	outPath := ""
	if changed {
		outPath = newPath
	}

	return checks.FixResult{
		Data:      a.Data,
		Path:      outPath,
		DidChange: changed,
		Note:      note,
	}, nil
}
