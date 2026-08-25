package checks

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
)

type CSVReader interface {
	Read() ([]string, error)
}

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

func NewCSVReader(data []byte, delim rune) *csv.Reader {
	br := bufio.NewReader(bytes.NewReader(data))

	r := csv.NewReader(br)
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	return r
}

func NewSemicolonCSVReader(data []byte) *csv.Reader {
	return NewCSVReader(data, ';')
}

func IsBlankCSVRecord(record []string) bool {
	for _, col := range record {
		if !IsBlankUnicode([]byte(col)) {
			return false
		}
	}

	return true
}

func ReadFirstNonBlankCSVRecord(
	ctx context.Context,
	r CSVReader,
) ([]string, error) {
	record, _, err := ReadFirstNonBlankCSVRecordWithRow(ctx, r)

	return record, err
}

func ReadFirstNonBlankCSVRecordWithRow(
	ctx context.Context,
	r CSVReader,
) ([]string, int, error) {
	rowNum := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil, rowNum, err
		}

		record, err := r.Read()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, rowNum, ctxErr
			}

			return nil, rowNum, err
		}

		rowNum++

		if !IsBlankCSVRecord(record) {
			return record, rowNum, nil
		}
	}
}

func ReadFirstNonBlankSemicolonHeader(
	ctx context.Context,
	a Artifact,
	emptyMessage string,
) ([]string, ValidationResult, bool) {
	r, res, ok := NewSemicolonCSVReaderWithCtx(
		ctx,
		a,
		emptyMessage,
	)
	if !ok {
		return nil, res, false
	}

	record, err := ReadFirstNonBlankCSVRecord(ctx, r)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, CancelledValidation(ctxErr), false
		}

		return nil, ValidationResult{
			OK:  false,
			Msg: "cannot parse header with semicolon delimiter",
			Err: err,
		}, false
	}

	return record, ValidationResult{}, true
}

func ReadFirstNonBlankHeaderWithRow(
	ctx context.Context,
	r CSVReader,
	noHeaderMessage string,
) ([]string, int, ValidationResult, bool) {
	header, rowNum, err := ReadFirstNonBlankCSVRecordWithRow(ctx, r)
	if err == nil {
		return header, rowNum, ValidationResult{}, true
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, rowNum, CancelledValidation(ctxErr), false
	}

	if errors.Is(err, io.EOF) {
		return nil, rowNum, ValidationResult{
			OK:  true,
			Msg: noHeaderMessage,
		}, false
	}

	return nil, rowNum, ValidationResult{
		OK:  false,
		Msg: "cannot parse header with semicolon delimiter",
		Err: err,
	}, false
}

func ReadSemicolonHeader(
	ctx context.Context,
	a Artifact,
	emptyMessage string,
) ([]string, ValidationResult, bool) {
	r, res, ok := NewSemicolonCSVReaderWithCtx(ctx, a, emptyMessage)
	if !ok {
		return nil, res, false
	}

	header, err := r.Read()
	if err != nil || len(header) == 0 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, CancelledValidation(ctxErr), false
		}

		return nil, ValidationResult{
			OK:  false,
			Msg: "cannot parse header with semicolon delimiter",
			Err: err,
		}, false
	}

	return header, ValidationResult{}, true
}

func ReadAllCSVRecords(
	ctx context.Context,
	r *csv.Reader,
) ([][]string, error) {
	var records [][]string

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		record, err := r.Read()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}

			if errors.Is(err, io.EOF) {
				return records, nil
			}

			return nil, err
		}

		records = append(records, record)
	}
}

func FindHeaderColumn(
	header []string,
	name string,
) int {
	name = NormalizeStr(name)

	for i, col := range header {
		if NormalizeStr(col) == name {
			return i
		}
	}

	return -1
}

func ReadFirstNonBlankHeader(
	ctx context.Context,
	r CSVReader,
	noHeaderMessage string,
) ([]string, ValidationResult, bool) {
	header, err := ReadFirstNonBlankCSVRecord(ctx, r)
	if err == nil {
		return header, ValidationResult{}, true
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, CancelledValidation(ctxErr), false
	}

	if errors.Is(err, io.EOF) {
		return nil, ValidationResult{
			OK:  true,
			Msg: noHeaderMessage,
		}, false
	}

	return nil, ValidationResult{
		OK:  false,
		Msg: "cannot parse header with semicolon delimiter",
		Err: err,
	}, false
}
