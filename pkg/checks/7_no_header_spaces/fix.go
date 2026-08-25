package no_spaces_in_header

import (
	"bytes"
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixNoSpacesInHeader(
	ctx context.Context,
	a checks.Artifact,
) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)

	if checks.IsBlankUnicode(in) {
		return checks.NoFix(a, "no usable content to trim header")
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	headerLine, rest := splitFirstLine(in)

	record, err := readHeaderForFix(headerLine)
	if err != nil {
		return checks.NoFix(a, "cannot parse header; skip")
	}

	if !trimHeaderRecord(record) {
		return checks.NoChange(a, "header already trimmed"), nil
	}

	// The rewritten header needs its trailing newline either when the
	// original file had one or when more content follows the header.
	keepHeaderFinal := keepFinal || len(rest) > 0

	newHeader, err := checks.WriteSemicolonCSVRecords(
		ctx,
		[][]string{record},
		lineSep,
		keepHeaderFinal,
	)
	if err != nil {
		return checks.NoChange(
			a,
			"failed to write trimmed header: "+err.Error(),
		), err
	}

	out := make([]byte, 0, len(bom)+len(newHeader)+len(rest))
	out = append(out, bom...)
	out = append(out, newHeader...)
	out = append(out, rest...)

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
