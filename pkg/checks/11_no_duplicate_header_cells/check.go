package duplicate_header_cells

import (
	"context"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "warn-duplicate-header-cells"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runWarnDuplicateHeaderCells,
		checks.WithPriority(11),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runWarnDuplicateHeaderCells(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateDuplicateHeaderCells,
		Fix:              fixDuplicateHeaderCells,
		PassMsg:          "no duplicate header columns",
		FixedMsg:         "removed duplicate header columns",
		AppliedMsg:       "auto-fix applied: removed duplicate header columns",
		StatusAfterFixed: checks.Pass,
		FailAs:           checks.Warn,
		StillBadMsg:      "header still contains duplicate columns after fix",
	})
}

func validateDuplicateHeaderCells(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{
			OK:  false,
			Msg: "validation cancelled",
			Err: err,
		}
	}

	raw := string(a.Data)
	if raw == "" {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no content to check for duplicate headers",
		}
	}

	lines := strings.Split(raw, "\n")
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no header line found (nothing to check for duplicates)",
		}
	}

	headerLine := lines[headerIdx]
	if strings.TrimSpace(headerLine) == "" {
		return checks.ValidationResult{
			OK:  true,
			Msg: "empty header line (nothing to check for duplicates)",
		}
	}

	colsRaw := strings.Split(headerLine, ";")

	type stat struct {
		Count  int
		Sample string
	}

	seen := make(map[string]*stat)

	for _, c := range colsRaw {
		trimmed := strings.TrimSpace(c)
		lc := strings.ToLower(trimmed)

		if s, ok := seen[lc]; ok {
			s.Count++
		} else {
			seen[lc] = &stat{
				Count:  1,
				Sample: trimmed,
			}
		}
	}

	var dups []string
	for _, st := range seen {
		if st.Count > 1 {
			dups = append(dups, st.Sample+"("+strconv.Itoa(st.Count)+")")
		}
	}

	if len(dups) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no duplicate header columns",
		}
	}

	msg := "duplicate header columns: " + strings.Join(dups, ", ")

	return checks.ValidationResult{
		OK:  false,
		Msg: msg,
	}
}
