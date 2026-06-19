package no_empty_term_values

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-empty-term-values"

const (
	ctxCheckEveryRows = 1 << 12
	maxReportedRows   = 10
)

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoEmptyTermValues,
		checks.WithFailFast(),
		checks.WithPriority(12),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoEmptyTermValues(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:     checkName,
		Validate: validateNoEmptyTermValues,
		Fix:      nil,
		PassMsg:  "all rows have non-empty term",
	})
}

func validateNoEmptyTermValues(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return cancelledValidation(err)
	}

	data := checks.StripUTF8BOM(a.Data)
	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no content to validate for empty term values",
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
			Msg: "no 'term' column found (skipping empty term validation)",
		}
	}

	badRows, err := findRowsWithEmptyTerm(ctx, r, rowNum, termCol)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return cancelledValidation(ctxErr)
		}

		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot parse CSV while validating empty term values",
			Err: err,
		}
	}

	if len(badRows) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "all rows have non-empty term",
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: emptyTermRowsMessage(badRows),
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
					Msg: "no header line found (nothing to validate for empty term values)",
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

func findRowsWithEmptyTerm(
	ctx context.Context,
	r csvReader,
	rowNum int,
	termCol int,
) ([]int, error) {
	var badRows []int

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

		if hasEmptyTermValue(rec, termCol) {
			badRows = append(badRows, rowNum)
		}
	}
}

func hasEmptyTermValue(record []string, termCol int) bool {
	if termCol >= len(record) {
		return true
	}

	return checks.IsBlankUnicode([]byte(record[termCol]))
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}

func emptyTermRowsMessage(rows []int) string {
	display := rows
	truncated := false

	if len(display) > maxReportedRows {
		display = display[:maxReportedRows]
		truncated = true
	}

	var b strings.Builder
	b.WriteString("empty term in rows: ")

	for i, row := range display {
		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(strconv.Itoa(row))
	}

	if truncated {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(rows)))
		b.WriteString(")")
		return b.String()
	}

	b.WriteString(" (total ")
	b.WriteString(strconv.Itoa(len(rows)))
	b.WriteString(")")

	return b.String()
}
