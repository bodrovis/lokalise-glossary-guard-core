package no_spaces_in_header

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixNoSpacesInHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		return noHeaderTrimFix(a, "no usable content to trim header"), checks.ErrNoFix
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	headerLine, rest := splitFirstLine(in)

	record, err := readHeaderForFix(headerLine)
	if err != nil {
		return noHeaderTrimFix(a, "cannot parse header; skip"), checks.ErrNoFix
	}

	if !trimHeaderRecord(record) {
		return noHeaderTrimFix(a, "header already trimmed"), nil
	}

	newHeader, err := writeHeaderRecord(record, lineSep, !keepFinal && len(rest) == 0)
	if err != nil {
		return noHeaderTrimFix(a, "failed to write trimmed header: "+err.Error()), err
	}

	out := make([]byte, 0, len(bom)+len(newHeader)+len(rest))
	out = append(out, bom...)
	out = append(out, newHeader...)
	out = append(out, rest...)

	if keepFinal && !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, []byte(lineSep)...)
	}

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "trimmed leading/trailing spaces in header cells",
	}, nil
}

func splitFirstLine(data []byte) ([]byte, []byte) {
	headerLine, rest, found := bytes.Cut(data, []byte("\n"))
	if !found {
		return data, nil
	}

	if len(headerLine) > 0 && headerLine[len(headerLine)-1] == '\r' {
		headerLine = headerLine[:len(headerLine)-1]
	}

	return headerLine, rest
}

func readHeaderForFix(headerLine []byte) ([]string, error) {
	r := checks.NewSemicolonCSVReader(headerLine)
	return r.Read()
}

func trimHeaderRecord(record []string) bool {
	changed := false

	for i, v := range record {
		trimmed := strings.TrimSpace(v)
		if v != trimmed {
			record[i] = trimmed
			changed = true
		}
	}

	return changed
}

func writeHeaderRecord(record []string, lineSep string, stripFinalNewline bool) ([]byte, error) {
	var hb bytes.Buffer

	w := csv.NewWriter(&hb)
	w.Comma = ';'

	if err := w.Write(record); err != nil {
		return nil, err
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	newHeader := hb.Bytes()
	if lineSep == "\r\n" {
		newHeader = bytes.ReplaceAll(newHeader, []byte("\n"), []byte("\r\n"))
	}

	if stripFinalNewline {
		newHeader = trimFinalCSVWriterNewline(newHeader)
	}

	return newHeader, nil
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

func noHeaderTrimFix(a checks.Artifact, note string) checks.FixResult {
	return checks.FixResult{
		Data:      a.Data,
		Path:      "",
		DidChange: false,
		Note:      note,
	}
}
