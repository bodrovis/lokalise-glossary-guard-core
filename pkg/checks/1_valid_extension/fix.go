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

	path := strings.TrimSpace(a.Path)
	if path == "" {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "empty path: nothing to fix",
		}, nil
	}

	ext := filepath.Ext(path)
	if strings.EqualFold(ext, ".csv") {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "already has .csv extension",
		}, nil
	}

	newPath := withCSVExtension(path, ext)

	return checks.FixResult{
		Data:      a.Data,
		Path:      newPath,
		DidChange: true,
		Note:      "renamed to .csv",
	}, nil
}

func withCSVExtension(path, ext string) string {
	base := strings.TrimSuffix(path, ext)
	base = strings.TrimSuffix(base, ".")

	return base + ".csv"
}
