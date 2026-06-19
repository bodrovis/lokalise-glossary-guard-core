package invalid_flags

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-invalid-flags"

const (
	ctxCheckEveryRows     = 1 << 12
	maxReportedFlagErrors = 10
)

var watchedCols = []string{
	"casesensitive",
	"translatable",
	"forbidden",
}

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoInvalidFlags,
		checks.WithFailFast(),
		checks.WithPriority(15),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoInvalidFlags(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateNoInvalidFlags,
		Fix:              fixNoInvalidFlags,
		PassMsg:          "all flag columns contain only yes/no",
		FixedMsg:         "normalized flag columns to yes/no",
		AppliedMsg:       "auto-fix applied: normalized flag columns to yes/no",
		StatusAfterFixed: checks.Pass,
		StillBadMsg:      "invalid flag values remain after fix",
	})
}

func validateNoInvalidFlags(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return cancelledValidation(err)
	}

	data := checks.StripUTF8BOM(a.Data)
	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no content to validate for flags",
		}
	}

	r := checks.NewSemicolonCSVReader(data)

	header, rowNum, res, ok := readFlagHeader(ctx, r)
	if !ok {
		return res
	}

	flagColumns := findFlagColumns(header)
	if len(flagColumns) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no flag columns found",
		}
	}

	invalids, err := findInvalidFlagValues(ctx, r, rowNum, flagColumns)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return cancelledValidation(ctxErr)
		}

		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot parse CSV while validating flag values",
			Err: err,
		}
	}

	if len(invalids) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "all flag columns contain only yes/no",
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: invalidFlagsMessage(invalids),
	}
}

type csvReader interface {
	Read() ([]string, error)
}

func readFlagHeader(
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
					Msg: "no header line found (nothing to validate for flags)",
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

type flagColumn struct {
	name string
	pos  int
}

func findFlagColumns(header []string) []flagColumn {
	watched := make(map[string]struct{}, len(watchedCols))
	for _, col := range watchedCols {
		watched[col] = struct{}{}
	}

	cols := make([]flagColumn, 0, len(watchedCols))

	for i, h := range header {
		name := normalizeHeaderCell(h)
		if _, ok := watched[name]; !ok {
			continue
		}

		cols = append(cols, flagColumn{
			name: name,
			pos:  i,
		})
	}

	return cols
}

func normalizeHeaderCell(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type invalidFlagValue struct {
	colName string
	value   string
	rowNum  int
}

func findInvalidFlagValues(
	ctx context.Context,
	r csvReader,
	rowNum int,
	flagColumns []flagColumn,
) ([]invalidFlagValue, error) {
	var invalids []invalidFlagValue

	for {
		if rowNum%ctxCheckEveryRows == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return invalids, nil
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

		for _, col := range flagColumns {
			value := flagValue(rec, col.pos)
			if isValidFlagValue(value) {
				continue
			}

			invalids = append(invalids, invalidFlagValue{
				colName: col.name,
				value:   value,
				rowNum:  rowNum,
			})
		}
	}
}

func flagValue(record []string, pos int) string {
	if pos >= len(record) {
		return ""
	}

	return strings.TrimSpace(record[pos])
}

func isValidFlagValue(value string) bool {
	return value == "yes" || value == "no"
}

func invalidFlagsMessage(invalids []invalidFlagValue) string {
	limit := len(invalids)
	if limit > maxReportedFlagErrors {
		limit = maxReportedFlagErrors
	}

	var b strings.Builder
	b.WriteString("invalid values in flag columns: ")

	for i := 0; i < limit; i++ {
		inv := invalids[i]

		b.WriteString(inv.colName)
		b.WriteString("=")
		b.WriteString(strconv.Quote(inv.value))
		b.WriteString(" (row ")
		b.WriteString(strconv.Itoa(inv.rowNum))
		b.WriteString(")")

		if i != limit-1 {
			b.WriteString("; ")
		}
	}

	if len(invalids) > limit {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(invalids)))
		b.WriteString(" invalid values)")
		return b.String()
	}

	b.WriteString(" (total ")
	b.WriteString(strconv.Itoa(len(invalids)))
	b.WriteString(" invalid values)")

	return b.String()
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
