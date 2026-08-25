package checks

import (
	"bytes"
	"context"
	"encoding/csv"
)

func WriteSemicolonCSVRecords(
	ctx context.Context,
	records [][]string,
	lineSep string,
	keepFinal bool,
) ([]byte, error) {
	var buf bytes.Buffer

	w := csv.NewWriter(&buf)
	w.Comma = ';'

	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := w.Write(record); err != nil {
			return nil, err
		}
	}

	w.Flush()

	if err := w.Error(); err != nil {
		return nil, err
	}

	out := buf.Bytes()

	if lineSep == "\r\n" {
		out = bytes.ReplaceAll(
			out,
			[]byte("\n"),
			[]byte("\r\n"),
		)
	}

	if !keepFinal {
		out = trimFinalCSVWriterNewline(out)
	}

	return out, nil
}

func trimFinalCSVWriterNewline(data []byte) []byte {
	if bytes.HasSuffix(data, []byte("\r\n")) {
		return data[:len(data)-2]
	}

	if bytes.HasSuffix(data, []byte("\n")) {
		return data[:len(data)-1]
	}

	return data
}
