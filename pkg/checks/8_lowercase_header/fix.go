package lowercase_header

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type headerLineParts struct {
	before []byte
	line   []byte
	rest   []byte
}

func fixLowercaseHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		return noLowercaseHeaderFix(a, "no usable content to normalize header"), checks.ErrNoFix
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	parts, ok, err := findHeaderLine(ctx, in)
	if err != nil {
		return checks.FixResult{}, err
	}
	if !ok {
		return noLowercaseHeaderFix(a, "no header line found"), checks.ErrNoFix
	}

	record, err := readHeaderRecordForFix(parts.line)
	if err != nil || len(record) == 0 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return checks.FixResult{}, ctxErr
		}

		return noLowercaseHeaderFix(a, "cannot parse header with semicolon delimiter"), checks.ErrNoFix
	}

	changed, err := lowercaseKnownHeaderColumns(ctx, record)
	if err != nil {
		return checks.FixResult{}, err
	}
	if !changed {
		return noLowercaseHeaderFix(a, "header service columns already lowercase"), nil
	}

	stripFinalNewline := !keepFinal && len(parts.rest) == 0

	newHeader, err := writeHeaderRecord(record, lineSep, stripFinalNewline)
	if err != nil {
		return noLowercaseHeaderFix(a, "failed to serialize normalized header"), err
	}

	out := stitchHeaderFix(bom, parts.before, newHeader, parts.rest, lineSep, keepFinal)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "normalized service columns in header to lowercase",
	}, nil
}

func findHeaderLine(ctx context.Context, data []byte) (headerLineParts, bool, error) {
	pos := 0

	for pos <= len(data) {
		if err := ctx.Err(); err != nil {
			return headerLineParts{}, false, err
		}

		line, rest, found := bytes.Cut(data[pos:], []byte("\n"))

		lineForCheck := trimTrailingCR(line)
		if !checks.IsBlankUnicode(lineForCheck) {
			headerEnd := len(data) - len(rest)
			if !found {
				headerEnd = len(data)
			}

			return headerLineParts{
				before: data[:pos],
				line:   lineForCheck,
				rest:   data[headerEnd:],
			}, true, nil
		}

		if !found {
			break
		}

		pos += len(line) + 1
	}

	return headerLineParts{}, false, nil
}

func trimTrailingCR(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}

	return line
}

func readHeaderRecordForFix(headerLine []byte) ([]string, error) {
	r := checks.NewSemicolonCSVReader(headerLine)
	return r.Read()
}

func lowercaseKnownHeaderColumns(ctx context.Context, record []string) (bool, error) {
	changed := false

	for i, col := range record {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		lower, ok := lowercaseKnownHeaderColumn(col)
		if !ok {
			continue
		}

		if record[i] != lower {
			record[i] = lower
			changed = true
		}
	}

	return changed, nil
}

func lowercaseKnownHeaderColumn(col string) (string, bool) {
	trimmed := strings.TrimSpace(col)
	if trimmed == "" {
		return "", false
	}

	lower := strings.ToLower(trimmed)
	if _, ok := checks.KnownHeaders[lower]; !ok {
		return "", false
	}

	return lower, true
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

func stitchHeaderFix(
	bom []byte,
	before []byte,
	newHeader []byte,
	rest []byte,
	lineSep string,
	keepFinal bool,
) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(newHeader)+len(rest)+len(lineSep))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, newHeader...)
	out = append(out, rest...)

	if keepFinal && !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, []byte(lineSep)...)
	}

	return out
}

func noLowercaseHeaderFix(a checks.Artifact, note string) checks.FixResult {
	return checks.FixResult{
		Data:      a.Data,
		Path:      "",
		DidChange: false,
		Note:      note,
	}
}
