package invalid_flags

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type flagFixInput struct {
	data      []byte
	bom       []byte
	lineSep   string
	keepFinal bool
	parts     flagFixHeaderParts
}

type flagFixHeaderParts struct {
	before []byte
	line   []byte
	rest   []byte
}

func fixNoInvalidFlags(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	prep, fr, ok, err := prepareFlagFixInput(ctx, a)
	if !ok || err != nil {
		return fr, err
	}

	records, fr, ok, err := parseFlagFixRecords(ctx, a, prep)
	if !ok || err != nil {
		return fr, err
	}

	outRecs, fr, ok, err := buildFlagFixOutput(ctx, a, records)
	if !ok || err != nil {
		return fr, err
	}

	return serializeFlagFixResult(ctx, a, prep, outRecs)
}

func prepareFlagFixInput(
	ctx context.Context,
	a checks.Artifact,
) (flagFixInput, checks.FixResult, bool, error) {
	if err := ctx.Err(); err != nil {
		return flagFixInput{}, checks.FixResult{}, false, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		fr, err := checks.NoFix(a, "no usable content to fix")
		return flagFixInput{}, fr, false, err
	}

	parts, ok, err := findFlagFixHeaderLine(ctx, in)
	if err != nil {
		return flagFixInput{}, checks.FixResult{}, false, err
	}
	if !ok {
		fr, err := checks.NoFix(a, "no header line found")
		return flagFixInput{}, fr, false, err
	}

	return flagFixInput{
		data:      in,
		bom:       bom,
		lineSep:   checks.DetectLineEnding(in),
		keepFinal: bytes.HasSuffix(in, []byte("\n")),
		parts:     parts,
	}, checks.FixResult{}, true, nil
}

func parseFlagFixRecords(
	ctx context.Context,
	a checks.Artifact,
	prep flagFixInput,
) ([][]string, checks.FixResult, bool, error) {
	records, err := readFlagFixRecords(ctx, appendFlagFixHeaderAndRest(prep.parts))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, checks.FixResult{}, false, ctxErr
		}

		fr, noFixErr := checks.NoFix(a, "cannot parse CSV with semicolon delimiter")
		return nil, fr, false, noFixErr
	}

	if len(records) == 0 || flagFixIsBlankCSVRecord(records[0]) {
		fr, err := checks.NoFix(a, "empty header line")
		return nil, fr, false, err
	}

	return records, checks.FixResult{}, true, nil
}

func buildFlagFixOutput(
	ctx context.Context,
	a checks.Artifact,
	records [][]string,
) ([][]string, checks.FixResult, bool, error) {
	flagColumns := flagFixColumns(records[0])
	if len(flagColumns) == 0 {
		return nil, flagFixNoChange(a, "no flag columns to normalize"), false, nil
	}

	outRecs, changed, err := normalizeFlagRecords(ctx, records, flagColumns)
	if err != nil {
		return nil, checks.FixResult{}, false, err
	}

	if !changed {
		return nil, flagFixNoChange(a, "no flag values to normalize"), false, nil
	}

	return outRecs, checks.FixResult{}, true, nil
}

func serializeFlagFixResult(
	ctx context.Context,
	a checks.Artifact,
	prep flagFixInput,
	outRecs [][]string,
) (checks.FixResult, error) {
	outTail, err := writeFlagFixRecords(ctx, outRecs, prep.lineSep, prep.keepFinal)
	if err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "failed to serialize CSV: " + err.Error(),
		}, err
	}

	out := stitchFlagFix(prep.bom, prep.parts.before, outTail)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "normalized flag columns to yes/no",
	}, nil
}

func flagFixNoChange(a checks.Artifact, note string) checks.FixResult {
	return checks.FixResult{
		Data:      a.Data,
		Path:      "",
		DidChange: false,
		Note:      note,
	}
}

func findFlagFixHeaderLine(
	ctx context.Context,
	data []byte,
) (flagFixHeaderParts, bool, error) {
	pos := 0

	for pos <= len(data) {
		if err := ctx.Err(); err != nil {
			return flagFixHeaderParts{}, false, err
		}

		line, rest, found := bytes.Cut(data[pos:], []byte("\n"))
		lineForCheck := flagFixTrimTrailingCR(line)

		if !checks.IsBlankUnicode(lineForCheck) {
			headerEnd := len(data) - len(rest)
			if !found {
				headerEnd = len(data)
			}

			return flagFixHeaderParts{
				before: data[:pos],
				line:   data[pos:headerEnd],
				rest:   data[headerEnd:],
			}, true, nil
		}

		if !found {
			break
		}

		pos += len(line) + 1
	}

	return flagFixHeaderParts{}, false, nil
}

func flagFixTrimTrailingCR(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}

	return line
}

func appendFlagFixHeaderAndRest(parts flagFixHeaderParts) []byte {
	out := make([]byte, 0, len(parts.line)+len(parts.rest))
	out = append(out, parts.line...)
	out = append(out, parts.rest...)

	return out
}

func readFlagFixRecords(ctx context.Context, data []byte) ([][]string, error) {
	r := checks.NewSemicolonCSVReader(data)

	var records [][]string

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return records, nil
			}

			return nil, err
		}

		records = append(records, rec)
	}
}

func writeFlagFixRecords(
	ctx context.Context,
	records [][]string,
	lineSep string,
	keepFinal bool,
) ([]byte, error) {
	var buf bytes.Buffer

	w := csv.NewWriter(&buf)
	w.Comma = ';'

	for i := range len(records) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := w.Write(records[i]); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	out := buf.Bytes()

	if lineSep == "\r\n" {
		out = bytes.ReplaceAll(out, []byte("\n"), []byte("\r\n"))
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

type flagFixColumn struct {
	name string
	pos  int
}

func flagFixColumns(header []string) []flagFixColumn {
	watched := make(map[string]struct{}, len(watchedCols))
	for _, col := range watchedCols {
		watched[col] = struct{}{}
	}

	cols := make([]flagFixColumn, 0, len(watchedCols))

	for i, h := range header {
		name := strings.ToLower(strings.TrimSpace(h))
		if _, ok := watched[name]; !ok {
			continue
		}

		cols = append(cols, flagFixColumn{
			name: name,
			pos:  i,
		})
	}

	return cols
}

func normalizeFlagRecords(
	ctx context.Context,
	records [][]string,
	flagColumns []flagFixColumn,
) ([][]string, bool, error) {
	out := make([][]string, len(records))
	out[0] = records[0]

	changed := false

	for i := 1; i < len(records); i++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}

		row := records[i]
		newRow := make([]string, len(row))
		copy(newRow, row)

		if flagFixIsBlankCSVRecord(row) {
			out[i] = newRow
			continue
		}

		for _, col := range flagColumns {
			if col.pos < 0 || col.pos >= len(newRow) {
				continue
			}

			orig := newRow[col.pos]
			normalized := normalizeFlagValue(orig)

			if normalized != orig {
				newRow[col.pos] = normalized
				changed = true
			}
		}

		out[i] = newRow
	}

	return out, changed, nil
}

func normalizeFlagValue(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return v
	}

	switch strings.ToLower(trimmed) {
	case "yes", "y", "true", "1":
		return "yes"
	case "no", "n", "false", "0":
		return "no"
	default:
		return v
	}
}

func flagFixIsBlankCSVRecord(record []string) bool {
	for _, field := range record {
		if !checks.IsBlankUnicode([]byte(field)) {
			return false
		}
	}

	return true
}

func stitchFlagFix(bom, before, tail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(tail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, tail...)

	return out
}
