package valid_extension

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-valid-extension"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureCSV,
		checks.WithFailFast(),
		checks.WithPriority(1),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

// runEnsureCSV: validate → maybe fix → maybe revalidate
func runEnsureCSV(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateCSVExt,
		Fix:              fixCSVExt,
		PassMsg:          "file extension OK: .csv",
		FixedMsg:         "extension fixed to .csv",
		AppliedMsg:       "auto-fix applied (renamed to .csv)",
		StatusAfterFixed: checks.Pass,
	})
}

func validateCSVExt(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	path := strings.TrimSpace(a.Path)
	if path == "" {
		return checks.ValidationResult{OK: false, Msg: "empty path: cannot validate extension"}
	}

	ext := filepath.Ext(path) // includes the leading dot, or "" if none
	if strings.EqualFold(ext, ".csv") {
		return checks.ValidationResult{OK: true}
	}

	if ext == "" {
		return checks.ValidationResult{OK: false, Msg: "invalid file extension: (none) (expected .csv)"}
	}
	return checks.ValidationResult{OK: false, Msg: "invalid file extension: " + ext + " (expected .csv)"}
}
