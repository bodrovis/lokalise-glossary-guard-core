package term_description_header

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type physicalHeaderParts struct {
	before []byte
	line   []byte
	rest   []byte
}

type termDescriptionPlan struct {
	hasTerm        bool
	hasDescription bool
	termIndex      int
	descIndex      int
	restIndexes    []int
	alreadyOK      bool
}

type termDescriptionFixSource struct {
	bom       []byte
	lineSep   string
	keepFinal bool
	parts     physicalHeaderParts
	records   [][]string
}

func (s termDescriptionFixSource) header() []string {
	if len(s.records) == 0 {
		return nil
	}

	return s.records[0]
}

type earlyTermDescriptionFix struct {
	result checks.FixResult
	err    error
}

func fixTermDescriptionHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	source, early, err := prepareTermDescriptionFix(ctx, a)
	if err != nil {
		return checks.FixResult{}, err
	}
	if early != nil {
		return early.result, early.err
	}

	plan := buildTermDescriptionPlan(source.header())
	if plan.alreadyOK {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "header already starts with term;description",
		}, nil
	}

	outRecs, err := applyTermDescriptionPlan(ctx, source.records, plan)
	if err != nil {
		return checks.FixResult{}, err
	}

	body, err := serializeTermDescriptionRecords(ctx, outRecs, source)
	if err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "failed to serialize CSV: " + err.Error(),
		}, err
	}

	return checks.FixResult{
		Data:      stitchFixedCSV(source.bom, source.parts.before, body),
		Path:      "",
		DidChange: true,
		Note:      termDescriptionFixNote(plan),
	}, nil
}

func prepareTermDescriptionFix(
	ctx context.Context,
	a checks.Artifact,
) (termDescriptionFixSource, *earlyTermDescriptionFix, error) {
	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		return termDescriptionFixSource{}, noFixEarly(a, "no usable content to fix"), nil
	}

	parts, ok, err := findFirstNonBlankPhysicalLine(ctx, in)
	if err != nil {
		return termDescriptionFixSource{}, nil, err
	}
	if !ok {
		return termDescriptionFixSource{}, noFixEarly(a, "no header line found"), nil
	}

	records, ok, err := readTermDescriptionRecords(ctx, parts)
	if err != nil {
		return termDescriptionFixSource{}, nil, err
	}
	if !ok {
		return termDescriptionFixSource{}, noFixEarly(a, "cannot parse CSV with semicolon delimiter"), nil
	}

	return termDescriptionFixSource{
		bom:       bom,
		lineSep:   checks.DetectLineEnding(in),
		keepFinal: bytes.HasSuffix(in, []byte("\n")),
		parts:     parts,
		records:   records,
	}, nil, nil
}

func noFixEarly(a checks.Artifact, note string) *earlyTermDescriptionFix {
	result, err := checks.NoFix(a, note)

	return &earlyTermDescriptionFix{
		result: result,
		err:    err,
	}
}

func readTermDescriptionRecords(
	ctx context.Context,
	parts physicalHeaderParts,
) ([][]string, bool, error) {
	data := appendHeaderAndRest(parts)

	records, err := readSemicolonRecords(ctx, data)
	if err != nil {
		return nil, false, err
	}

	if len(records) == 0 || len(records[0]) == 0 {
		return nil, false, nil
	}

	return records, true, nil
}

func appendHeaderAndRest(parts physicalHeaderParts) []byte {
	data := make([]byte, 0, len(parts.line)+len(parts.rest))
	data = append(data, parts.line...)
	data = append(data, parts.rest...)

	return data
}

func serializeTermDescriptionRecords(
	ctx context.Context,
	records [][]string,
	source termDescriptionFixSource,
) ([]byte, error) {
	body, err := writeSemicolonRecords(ctx, records)
	if err != nil {
		return nil, err
	}

	if source.lineSep == "\r\n" {
		body = bytes.ReplaceAll(body, []byte("\n"), []byte("\r\n"))
	}

	if !source.keepFinal {
		body = trimFinalCSVWriterNewline(body)
	}

	return body, nil
}

func termDescriptionFixNote(plan termDescriptionPlan) string {
	if !plan.hasTerm || !plan.hasDescription {
		return "inserted missing term/description columns at start"
	}

	return "reordered columns to start with term;description"
}

func findFirstNonBlankPhysicalLine(ctx context.Context, data []byte) (physicalHeaderParts, bool, error) {
	pos := 0

	for pos <= len(data) {
		if err := ctx.Err(); err != nil {
			return physicalHeaderParts{}, false, err
		}

		line, rest, found := bytes.Cut(data[pos:], []byte("\n"))
		lineForCheck := trimTrailingCR(line)

		if !checks.IsBlankUnicode(lineForCheck) {
			headerEnd := len(data) - len(rest)
			if !found {
				headerEnd = len(data)
			}

			return physicalHeaderParts{
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

	return physicalHeaderParts{}, false, nil
}

func trimTrailingCR(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}

	return line
}

func buildTermDescriptionPlan(header []string) termDescriptionPlan {
	plan := termDescriptionPlan{
		termIndex: -1,
		descIndex: -1,
	}

	for i, col := range header {
		switch normalizeHeaderCell(col) {
		case "term":
			if plan.termIndex < 0 {
				plan.termIndex = i
				plan.hasTerm = true
			}
		case "description":
			if plan.descIndex < 0 {
				plan.descIndex = i
				plan.hasDescription = true
			}
		}
	}

	for i := range header {
		if i == plan.termIndex || i == plan.descIndex {
			continue
		}

		plan.restIndexes = append(plan.restIndexes, i)
	}

	plan.alreadyOK = len(header) >= 2 &&
		normalizeHeaderCell(header[0]) == "term" &&
		normalizeHeaderCell(header[1]) == "description"

	return plan
}

func applyTermDescriptionPlan(
	ctx context.Context,
	records [][]string,
	plan termDescriptionPlan,
) ([][]string, error) {
	out := make([][]string, len(records))

	for i, row := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		newRow := make([]string, 2+len(plan.restIndexes))

		if plan.termIndex >= 0 && plan.termIndex < len(row) {
			newRow[0] = row[plan.termIndex]
		}
		if plan.descIndex >= 0 && plan.descIndex < len(row) {
			newRow[1] = row[plan.descIndex]
		}

		for j, oldIndex := range plan.restIndexes {
			if oldIndex < len(row) {
				newRow[2+j] = row[oldIndex]
			}
		}

		out[i] = newRow
	}

	out[0][0] = "term"
	out[0][1] = "description"

	return out, nil
}

func readSemicolonRecords(ctx context.Context, data []byte) ([][]string, error) {
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

func writeSemicolonRecords(ctx context.Context, records [][]string) ([]byte, error) {
	var buf bytes.Buffer

	w := csv.NewWriter(&buf)
	w.Comma = ';'

	for _, rec := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := w.Write(rec); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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

func stitchFixedCSV(bom, before, body []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(body))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, body...)

	return out
}
