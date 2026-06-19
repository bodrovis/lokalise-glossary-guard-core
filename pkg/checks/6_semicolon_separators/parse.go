package semicolon_separator

import (
	"context"
	"errors"
	"io"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// attemptRectParse tries to parse data with the given delimiter using encoding/csv
// and then validates that the result is a proper "table":
// - at least one non-empty record
// - every record has the same number of fields
// - that number of fields > 1
//
// It returns false for parser errors because that delimiter simply does not
// cleanly parse the file. Context cancellation is returned as an error.
func attemptRectParse(ctx context.Context, data []byte, delim rune) (bool, error) {
	_, ok, err := parseRectRecords(ctx, data, delim)
	return ok, err
}

func parseRectRecords(ctx context.Context, data []byte, delim rune) ([][]string, bool, error) {
	recs, err := readCSVRecords(ctx, data, delim)
	if err != nil {
		return nil, false, err
	}
	if len(recs) == 0 {
		return nil, false, nil
	}

	width := firstTableWidth(recs)
	if width <= 1 {
		return nil, false, nil
	}

	if !allRecordsHaveWidth(recs, width) {
		return nil, false, nil
	}

	return recs, true, nil
}

func readCSVRecords(ctx context.Context, data []byte, delim rune) ([][]string, error) {
	r := checks.NewCSVReader(data, delim)

	var recs [][]string

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return recs, nil
		}
		if err != nil {
			// CSV parser failure means this delimiter does not cleanly parse the file.
			return nil, nil
		}

		recs = append(recs, rec)
	}
}

func firstTableWidth(recs [][]string) int {
	for _, row := range recs {
		if isBlankRecord(row) {
			continue
		}

		if len(row) > 1 {
			return len(row)
		}

		return 1
	}

	return 0
}

func allRecordsHaveWidth(recs [][]string, width int) bool {
	for _, row := range recs {
		if isBlankRecord(row) {
			continue
		}

		if len(row) != width {
			return false
		}
	}

	return true
}

func isBlankRecord(row []string) bool {
	if len(row) == 0 {
		return true
	}

	for _, field := range row {
		if !checks.IsBlankUnicode([]byte(field)) {
			return false
		}
	}

	return true
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
