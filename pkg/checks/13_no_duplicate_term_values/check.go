package duplicate_term_values

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const (
	ctxCheckEveryRows = 1 << 12
	maxReportedTerms  = 10
	checkName         = "warn-duplicate-term-values"
)

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runWarnDuplicateTermValues,
		checks.WithPriority(13),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runWarnDuplicateTermValues(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateWarnDuplicateTermValues,
		Fix:              fixDuplicateTermValues,
		PassMsg:          "no duplicate term values",
		FixedMsg:         "removed duplicate term rows",
		AppliedMsg:       "auto-fix applied: removed duplicate term rows",
		StatusAfterFixed: checks.Pass,
		FailAs:           checks.Warn,
		StillBadMsg:      "duplicate term values are still present after fix",
	})
}

// validateWarnDuplicateTermValues scans the "term" column and warns if the same non-empty value appears in multiple rows.
// Case-sensitive: "Apple" and "apple" are considered different terms.
// We report up to 10 offending term groups in the message, each annotated with row numbers (1-based).
func validateWarnDuplicateTermValues(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return cancelledValidation(err)
	}

	data := checks.StripUTF8BOM(a.Data)
	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no content to validate for duplicate term values",
		}
	}

	r := checks.NewSemicolonCSVReader(data)

	header, rowNum, res, ok := readTermHeader(ctx, r)
	if !ok {
		return res
	}

	termCol := findTermColumn(header)
	if termCol < 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no 'term' column found (skipping duplicate term check)",
		}
	}

	dups, err := findDuplicateTerms(ctx, r, rowNum, termCol)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return cancelledValidation(ctxErr)
		}

		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot parse CSV while validating duplicate term values",
			Err: err,
		}
	}

	if len(dups) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no duplicate term values",
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: duplicateTermsMessage(dups),
	}
}

type csvReader interface {
	Read() ([]string, error)
}

func readTermHeader(
	ctx context.Context,
	r csvReader,
) ([]string, int, checks.ValidationResult, bool) {
	rowNum := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil, rowNum, cancelledValidation(err), false
		}

		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, rowNum, checks.ValidationResult{
					OK:  true,
					Msg: "no header line found (nothing to validate for duplicate term values)",
				}, false
			}

			return nil, rowNum, checks.ValidationResult{
				OK:  false,
				Msg: "cannot parse header with semicolon delimiter",
				Err: err,
			}, false
		}

		rowNum++

		if !isBlankCSVRecord(rec) {
			return rec, rowNum, checks.ValidationResult{}, true
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

func findTermColumn(header []string) int {
	for i, col := range header {
		if normalizeHeaderCell(col) == "term" {
			return i
		}
	}

	return -1
}

func normalizeHeaderCell(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type duplicateTermInfo struct {
	term string
	rows []int
}

type termRows struct {
	rows     []int
	reported bool
}

func findDuplicateTerms(
	ctx context.Context,
	r csvReader,
	rowNum int,
	termCol int,
) ([]duplicateTermInfo, error) {
	seen := make(map[string]*termRows)
	var duplicateOrder []string

	for {
		if rowNum%ctxCheckEveryRows == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}

			return nil, err
		}

		rowNum++

		if isBlankCSVRecord(rec) {
			continue
		}

		term, ok := termValue(rec, termCol)
		if !ok {
			continue
		}

		entry := seen[term]
		if entry == nil {
			seen[term] = &termRows{
				rows: []int{rowNum},
			}
			continue
		}

		entry.rows = append(entry.rows, rowNum)
		if !entry.reported {
			duplicateOrder = append(duplicateOrder, term)
			entry.reported = true
		}
	}

	dups := make([]duplicateTermInfo, 0, len(duplicateOrder))
	for _, term := range duplicateOrder {
		dups = append(dups, duplicateTermInfo{
			term: term,
			rows: seen[term].rows,
		})
	}

	return dups, nil
}

func termValue(record []string, termCol int) (string, bool) {
	if termCol >= len(record) {
		return "", false
	}

	term := strings.TrimSpace(record[termCol])
	if term == "" {
		return "", false
	}

	return term, true
}

func duplicateTermsMessage(dups []duplicateTermInfo) string {
	limit := len(dups)
	if limit > maxReportedTerms {
		limit = maxReportedTerms
	}

	var b strings.Builder
	b.WriteString("duplicate term values found: ")

	for i := 0; i < limit; i++ {
		dup := dups[i]

		b.WriteString(strconv.Quote(dup.term))
		b.WriteString(" (rows ")
		b.WriteString(joinIntSlice(dup.rows, ", "))
		b.WriteString(")")

		if i != limit-1 {
			b.WriteString("; ")
		}
	}

	if len(dups) > limit {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(dups)))
		b.WriteString(" duplicate terms)")
		return b.String()
	}

	b.WriteString(" (total ")
	b.WriteString(strconv.Itoa(len(dups)))
	b.WriteString(" duplicate terms)")

	return b.String()
}

func joinIntSlice(nums []int, sep string) string {
	if len(nums) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(strconv.Itoa(nums[0]))

	for _, n := range nums[1:] {
		b.WriteString(sep)
		b.WriteString(strconv.Itoa(n))
	}

	return b.String()
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
