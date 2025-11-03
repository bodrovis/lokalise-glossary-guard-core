package valid_extension

import (
	"context"
	"fmt"
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

	base := strings.TrimSpace(a.Path)
	if base == "" {
		return checks.ValidationResult{
			OK:  false,
			Msg: "empty path: cannot validate extension",
		}
	}

	ext := filepath.Ext(base)
	if strings.EqualFold(ext, ".csv") {
		return checks.ValidationResult{OK: true, Msg: `extension is ".csv"`}
	}

	if ext == "" {
		return checks.ValidationResult{
			OK:  false,
			Msg: `invalid file extension: "" (expected ".csv")`,
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: fmt.Sprintf(`invalid file extension: %q (expected ".csv")`, ext),
	}
}
