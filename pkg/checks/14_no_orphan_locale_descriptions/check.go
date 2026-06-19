package orphan_locale_descriptions

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "warn-orphan-locale-descriptions"

const maxReportedOrphans = 10

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runWarnOrphanLocaleDescriptions,
		checks.WithPriority(14),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

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

func validateWarnOrphanLocaleDescriptions(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return cancelledValidation(err)
	}

	data := checks.StripUTF8BOM(a.Data)
	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no content to validate for orphan locale descriptions",
		}
	}

	r := checks.NewSemicolonCSVReader(data)

	header, res, ok := readOrphanLocaleHeader(ctx, r)
	if !ok {
		return res
	}

	orphans, err := findOrphanLocaleDescriptions(ctx, header)
	if err != nil {
		return cancelledValidation(err)
	}

	if len(orphans) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no orphan *_description columns",
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: orphanLocaleDescriptionsMessage(orphans),
	}
}

type csvReader interface {
	Read() ([]string, error)
}

func readOrphanLocaleHeader(
	ctx context.Context,
	r csvReader,
) ([]string, checks.ValidationResult, bool) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, cancelledValidation(err), false
		}

		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, checks.ValidationResult{
					OK:  true,
					Msg: "no header line found (nothing to validate for orphan locale descriptions)",
				}, false
			}

			return nil, checks.ValidationResult{
				OK:  false,
				Msg: "cannot parse header with semicolon delimiter",
				Err: err,
			}, false
		}

		if !isBlankCSVRecord(rec) {
			return rec, checks.ValidationResult{}, true
		}
	}
}

func isBlankCSVRecord(record []string) bool {
	for _, field := range record {
		if !checks.IsBlankUnicode([]byte(field)) {
			return false
		}
	}

	return true
}

func findOrphanLocaleDescriptions(ctx context.Context, header []string) ([]string, error) {
	allCols := make(map[string]struct{}, len(header))
	var candidateOrder []string
	seenCandidates := make(map[string]struct{})

	for _, col := range header {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name := normalizeHeaderCell(col)
		if name == "" {
			continue
		}

		allCols[name] = struct{}{}

		base, ok := descriptionBase(name)
		if !ok {
			continue
		}

		if _, seen := seenCandidates[base]; seen {
			continue
		}

		seenCandidates[base] = struct{}{}
		candidateOrder = append(candidateOrder, base)
	}

	orphans := make([]string, 0)

	for _, base := range candidateOrder {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if _, ok := allCols[base]; !ok {
			orphans = append(orphans, base)
		}
	}

	return orphans, nil
}

func normalizeHeaderCell(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func descriptionBase(name string) (string, bool) {
	if !strings.HasSuffix(name, "_description") {
		return "", false
	}

	base := strings.TrimSuffix(name, "_description")
	base = strings.TrimSpace(base)

	if base == "" {
		return "", false
	}

	return base, true
}

func orphanLocaleDescriptionsMessage(orphans []string) string {
	display := orphans
	truncated := false

	if len(display) > maxReportedOrphans {
		display = display[:maxReportedOrphans]
		truncated = true
	}

	var b strings.Builder
	b.WriteString("orphan *_description columns without matching base locale: ")
	b.WriteString(strings.Join(display, ", "))

	if truncated {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(orphans)))
		b.WriteString(")")
		return b.String()
	}

	b.WriteString(" (total ")
	b.WriteString(strconv.Itoa(len(orphans)))
	b.WriteString(")")

	return b.String()
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
