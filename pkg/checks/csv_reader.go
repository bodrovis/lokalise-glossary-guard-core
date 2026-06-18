package checks

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
)

func NewSemicolonCSVReaderWithCtx(
	ctx context.Context,
	a Artifact,
	emptyMessage string,
) (*csv.Reader, ValidationResult, bool) {
	if err := ctx.Err(); err != nil {
		return nil, ValidationResult{
			OK:  false,
			Msg: "validation cancelled",
			Err: err,
		}, false
	}

	if len(bytes.TrimSpace(a.Data)) == 0 {
		if emptyMessage == "" {
			emptyMessage = "no usable content"
		}

		return nil, ValidationResult{
			OK:  false,
			Msg: emptyMessage,
		}, false
	}

	reader := NewSemicolonCSVReader(a.Data)

	return reader, ValidationResult{}, true
}

func NewSemicolonCSVReader(data []byte) *csv.Reader {
	br := bufio.NewReader(bytes.NewReader(data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	return r
}
