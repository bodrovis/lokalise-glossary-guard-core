package orphan_locale_descriptions

import (
	"context"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "warn-orphan-locale-descriptions"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runWarnOrphanLocaleDescriptions,
		// not fail-fast, this is advisory
		checks.WithPriority(14),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

// runWarnOrphanLocaleDescriptions: validation + safe autofix.
func runWarnOrphanLocaleDescriptions(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateWarnOrphanLocaleDescriptions,
		Fix:              fixOrphanLocaleDescriptions,
		PassMsg:          "no orphan *_description columns",
		FixedMsg:         "added missing locale columns before *_description",
		AppliedMsg:       "auto-fix applied: added missing locale columns before *_description",
		StatusAfterFixed: checks.Pass,
		FailAs:           checks.Warn,
		StillBadMsg:      "orphan *_description columns remain after fix",
	})
}

// validateWarnOrphanLocaleDescriptions checks header columns looking for patterns like
// "<locale>_description" where there is no "<locale>" column in the same header.
// Example bad: "en_description" exists but "en" doesn't.
func validateWarnOrphanLocaleDescriptions(ctx context.Context, a checks.Artifact) checks.ValidationResult {
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
			Msg: "no content to validate for orphan locale descriptions",
		}
	}

	lines := strings.Split(raw, "\n")
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no header line found (nothing to validate for orphan locale descriptions)",
		}
	}

	headerLine := lines[headerIdx]
	if strings.TrimSpace(headerLine) == "" {
		return checks.ValidationResult{
			OK:  true,
			Msg: "empty header line (nothing to validate for orphan locale descriptions)",
		}
	}

	cols := checks.SplitHeaderCells(headerLine)

	// collect all column names lowercase
	allColsLC := make(map[string]struct{})
	// collect bases for *_description
	orphanCandidates := make(map[string]struct{}) // base -> seen <base>_description

	for _, c := range cols {
		nameTrim := strings.TrimSpace(c)
		lc := strings.ToLower(nameTrim)

		if lc == "" {
			continue
		}

		allColsLC[lc] = struct{}{}

		if strings.HasSuffix(lc, "_description") {
			base := strings.TrimSuffix(lc, "_description")
			base = strings.TrimSpace(base)
			if base != "" {
				orphanCandidates[base] = struct{}{}
			}
		}
	}

	var orphans []string
	for base := range orphanCandidates {
		if _, ok := allColsLC[base]; !ok {
			// we have "<base>_description", but no "<base>"
			orphans = append(orphans, base)
		}
	}

	if len(orphans) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no orphan *_description columns",
		}
	}

	// message (show up to 10)
	display := orphans
	if len(display) > 10 {
		display = display[:10]
	}

	var b strings.Builder
	b.WriteString("orphan *_description columns without matching base locale: ")
	b.WriteString(strings.Join(display, ", "))

	if len(orphans) > len(display) {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(orphans)))
		b.WriteString(")")
	} else {
		b.WriteString(" (total ")
		b.WriteString(strconv.Itoa(len(orphans)))
		b.WriteString(")")
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: b.String(),
	}
}
