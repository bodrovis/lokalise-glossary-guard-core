package semicolon_separator

import (
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// attemptRectParse tries to parse data with the given delimiter using encoding/csv
// and then validates that the result is a proper "table":
// - at least one non-empty record
// - every record has the same number of fields
// - that number of fields > 1
//
// Parser errors mean that the delimiter does not cleanly parse the file.
// Context cancellation is returned as an error.
func attemptRectParse(
	ctx context.Context,
	data []byte,
	delim rune,
) (bool, error) {
	_, ok, err := parseRectRecords(ctx, data, delim)
	return ok, err
}

func parseRectRecords(
	ctx context.Context,
	data []byte,
	delim rune,
) ([][]string, bool, error) {
	recs, err := readRectCSVRecords(ctx, data, delim)
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

func readRectCSVRecords(
	ctx context.Context,
	data []byte,
	delim rune,
) ([][]string, error) {
	r := checks.NewCSVReader(data, delim)

	recs, err := checks.ReadAllCSVRecords(ctx, r)
	if err == nil {
		return recs, nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	// A CSV parser error only means this delimiter is not suitable.
	return nil, nil
}

func firstTableWidth(recs [][]string) int {
	for _, row := range recs {
		if checks.IsBlankCSVRecord(row) {
			continue
		}

		if len(row) > 1 {
			return len(row)
		}

		return 1
	}

	return 0
}

func allRecordsHaveWidth(
	recs [][]string,
	width int,
) bool {
	for _, row := range recs {
		if checks.IsBlankCSVRecord(row) {
			continue
		}

		if len(row) != width {
			return false
		}
	}

	return true
}
