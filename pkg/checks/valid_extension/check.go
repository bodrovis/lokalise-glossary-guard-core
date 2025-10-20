package valid_extension

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-valid-extension"

var fixer checks.FixFunc = fixCSVExt

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureCSV,
		checks.WithFailFast(),
		checks.WithPriority(1),
		checks.WithRecover(),
	)
	if err != nil {
		panic("ensure-valid-extension: " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic("ensure-valid-extension register: " + err.Error())
	}
}

// runEnsureCSV: validate → maybe fix → maybe revalidate
func runEnsureCSV(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateCSVExt,
		Fix:              fixer,
		PassMsg:          "file extension OK: .csv",
		FixedMsg:         "extension fixed to .csv",
		AppliedMsg:       "auto-fix applied (renamed to .csv)",
		StatusAfterFixed: checks.Pass,
	})
}

func validateCSVExt(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{
			OK:  false,
			Msg: "validation cancelled",
			Err: err,
		}
	}

	path := strings.TrimSpace(a.Path)
	if path == "" {
		return checks.ValidationResult{
			OK:  false,
			Msg: "empty path: cannot validate extension",
			Err: nil,
		}
	}

	ext := filepath.Ext(path)
	if strings.EqualFold(ext, ".csv") {
		return checks.ValidationResult{
			OK:  true,
			Msg: "",
			Err: nil,
		}
	}

	if ext == "" {
		return checks.ValidationResult{
			OK:  false,
			Msg: "invalid file extension: (none) (expected .csv)",
			Err: nil,
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: "invalid file extension: " + ext + " (expected .csv)",
		Err: nil,
	}
}
