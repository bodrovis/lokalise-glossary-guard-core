package duplicate_header_cells

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixDuplicateHeaderCells(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		return checks.NoFix(a, "no usable content to fix")
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	headerStart, ok, err := findDuplicateHeaderStart(ctx, in)
	if err != nil {
		return checks.FixResult{}, err
	}
	if !ok {
		return checks.NoFix(a, "no header line found")
	}

	before := in[:headerStart]
	after := in[headerStart:]

	records, err := readDuplicateHeaderRecords(ctx, after)
	if err != nil {
		return checks.FixResult{}, err
	}
	if len(records) == 0 || len(records[0]) == 0 {
		return checks.NoFix(a, "cannot parse CSV with semicolon delimiter")
	}

	plan := buildDuplicateHeaderPlan(records[0])
	if !plan.hasDuplicates() {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no duplicate header columns to remove",
		}, nil
	}

	outRecs, err := applyDuplicateHeaderPlan(ctx, records, plan)
	if err != nil {
		return checks.FixResult{}, err
	}

	outTail, err := writeDuplicateHeaderRecords(ctx, outRecs, lineSep, keepFinal)
	if err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "failed to serialize CSV: " + err.Error(),
		}, err
	}

	out := stitchDuplicateHeaderFix(bom, before, outTail)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "removed duplicate header columns: " + strings.Join(plan.removedNames, ", "),
	}, nil
}

type duplicateHeaderPlan struct {
	keepIdx      []int
	removedNames []string
}

func (p duplicateHeaderPlan) hasDuplicates() bool {
	return len(p.removedNames) > 0
}

func findDuplicateHeaderStart(ctx context.Context, data []byte) (int, bool, error) {
	pos := 0

	for pos <= len(data) {
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}

		line, _, found := bytes.Cut(data[pos:], []byte("\n"))
		line = trimTrailingCR(line)

		if !checks.IsBlankUnicode(line) {
			return pos, true, nil
		}

		if !found {
			break
		}

		pos += len(line) + 1
	}

	return 0, false, nil
}

func readDuplicateHeaderRecords(ctx context.Context, data []byte) ([][]string, error) {
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

func buildDuplicateHeaderPlan(header []string) duplicateHeaderPlan {
	seen := make(map[string]struct{}, len(header))

	plan := duplicateHeaderPlan{
		keepIdx:      make([]int, 0, len(header)),
		removedNames: make([]string, 0),
	}

	for i, col := range header {
		key := duplicateHeaderKey(col)
		if _, ok := seen[key]; ok {
			plan.removedNames = append(plan.removedNames, duplicateHeaderSample(col))
			continue
		}

		seen[key] = struct{}{}
		plan.keepIdx = append(plan.keepIdx, i)
	}

	return plan
}

func applyDuplicateHeaderPlan(
	ctx context.Context,
	records [][]string,
	plan duplicateHeaderPlan,
) ([][]string, error) {
	out := make([][]string, len(records))

	for i, row := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		newRow := make([]string, len(plan.keepIdx))
		for j, idx := range plan.keepIdx {
			if idx < len(row) {
				newRow[j] = row[idx]
			}
		}

		out[i] = newRow
	}

	return out, nil
}

func writeDuplicateHeaderRecords(
	ctx context.Context,
	records [][]string,
	lineSep string,
	keepFinal bool,
) ([]byte, error) {
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

func stitchDuplicateHeaderFix(bom, before, tail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(tail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, tail...)

	return out
}

func trimTrailingCR(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}

	return line
}
