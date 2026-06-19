package no_forbidden_non_translatable_terms

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-forbidden-non-translatable-terms"

const (
	ctxCheckEveryRows = 1 << 12
	maxReportedRows   = 10
)

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoForbiddenNonTranslatableTerms,
		checks.WithFailFast(),
		checks.WithPriority(16),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoForbiddenNonTranslatableTerms(
	ctx context.Context,
	a checks.Artifact,
	opts checks.RunOptions,
) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:     checkName,
		Validate: validateNoForbiddenNonTranslatableTerms,
		Fix:      nil,
		PassMsg:  "no forbidden non-translatable terms found",
	})
}

func validateNoForbiddenNonTranslatableTerms(
	ctx context.Context,
	a checks.Artifact,
) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return cancelledValidation(err)
	}

	data := checks.StripUTF8BOM(a.Data)
	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no content to validate for forbidden non-translatable terms",
		}
	}

	r := checks.NewSemicolonCSVReader(data)

	header, rowNum, res, ok := readForbiddenNonTranslatableHeader(ctx, r)
	if !ok {
		return res
	}

	cols := findForbiddenNonTranslatableColumns(header)
	if !cols.hasRequiredFlags() {
		return checks.ValidationResult{
			OK:  true,
			Msg: "translatable or forbidden column not found (skipping forbidden non-translatable validation)",
		}
	}

	badRows, err := findForbiddenNonTranslatableRows(ctx, r, rowNum, cols)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return cancelledValidation(ctxErr)
		}

		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot parse CSV while validating forbidden non-translatable terms",
			Err: err,
		}
	}

	if len(badRows) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no forbidden non-translatable terms found",
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: forbiddenNonTranslatableMessage(badRows),
	}
}

type csvReader interface {
	Read() ([]string, error)
}

func readForbiddenNonTranslatableHeader(
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
					Msg: "no header line found (nothing to validate for forbidden non-translatable terms)",
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

type forbiddenNonTranslatableColumns struct {
	term         int
	translatable int
	forbidden    int
}

func findForbiddenNonTranslatableColumns(header []string) forbiddenNonTranslatableColumns {
	cols := forbiddenNonTranslatableColumns{
		term:         -1,
		translatable: -1,
		forbidden:    -1,
	}

	for i, h := range header {
		switch normalizeHeaderCell(h) {
		case "term":
			cols.term = i
		case "translatable":
			cols.translatable = i
		case "forbidden":
			cols.forbidden = i
		}
	}

	return cols
}

func (c forbiddenNonTranslatableColumns) hasRequiredFlags() bool {
	return c.translatable >= 0 && c.forbidden >= 0
}

func normalizeHeaderCell(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type forbiddenNonTranslatableRow struct {
	rowNum int
	term   string
}

func findForbiddenNonTranslatableRows(
	ctx context.Context,
	r csvReader,
	rowNum int,
	cols forbiddenNonTranslatableColumns,
) ([]forbiddenNonTranslatableRow, error) {
	var badRows []forbiddenNonTranslatableRow

	for {
		if rowNum%ctxCheckEveryRows == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return badRows, nil
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

		if !isForbiddenNonTranslatableRecord(rec, cols) {
			continue
		}

		badRows = append(badRows, forbiddenNonTranslatableRow{
			rowNum: rowNum,
			term:   recordValue(rec, cols.term),
		})
	}
}

func isForbiddenNonTranslatableRecord(
	record []string,
	cols forbiddenNonTranslatableColumns,
) bool {
	return recordValue(record, cols.translatable) == "no" &&
		recordValue(record, cols.forbidden) == "yes"
}

func recordValue(record []string, pos int) string {
	if pos < 0 || pos >= len(record) {
		return ""
	}

	return strings.TrimSpace(record[pos])
}

func forbiddenNonTranslatableMessage(rows []forbiddenNonTranslatableRow) string {
	limit := len(rows)
	if limit > maxReportedRows {
		limit = maxReportedRows
	}

	var b strings.Builder
	b.WriteString("terms cannot be both forbidden and non-translatable: ")

	for i := range limit {
		row := rows[i]

		if row.term != "" {
			b.WriteString("term=")
			b.WriteString(strconv.Quote(row.term))
			b.WriteString(" ")
		}

		b.WriteString("(row ")
		b.WriteString(strconv.Itoa(row.rowNum))
		b.WriteString(")")

		if i != limit-1 {
			b.WriteString("; ")
		}
	}

	if len(rows) > limit {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(rows)))
		b.WriteString(" terms)")
		return b.String()
	}

	b.WriteString(" (total ")
	b.WriteString(strconv.Itoa(len(rows)))
	b.WriteString(" terms)")

	return b.String()
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
