package orphan_locale_descriptions

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixOrphanLocaleDescriptions(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		return checks.NoFix(a, "no usable content to fix")
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	parts, ok, err := findOrphanFixHeaderLine(ctx, in)
	if err != nil {
		return checks.FixResult{}, err
	}
	if !ok {
		return checks.NoFix(a, "no header line found")
	}

	records, err := readOrphanFixRecords(ctx, appendOrphanFixHeaderAndRest(parts))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return checks.FixResult{}, ctxErr
		}

		return checks.NoFix(a, "cannot parse CSV with semicolon delimiter")
	}
	if len(records) == 0 || orphanFixIsBlankCSVRecord(records[0]) {
		return checks.NoFix(a, "empty header line")
	}

	plan := buildOrphanFixPlan(records[0])
	if !plan.hasChanges() {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no orphan *_description columns to fix",
		}, nil
	}

	outRecs, err := applyOrphanFixPlan(ctx, records, plan)
	if err != nil {
		return checks.FixResult{}, err
	}

	outTail, err := writeOrphanFixRecords(ctx, outRecs, lineSep, keepFinal)
	if err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "failed to serialize CSV: " + err.Error(),
		}, err
	}

	out := stitchOrphanFix(bom, parts.before, outTail)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "added missing locale columns before *_description: " + strings.Join(plan.insertedBases, ", "),
	}, nil
}

type orphanFixHeaderParts struct {
	before []byte
	line   []byte
	rest   []byte
}

func findOrphanFixHeaderLine(
	ctx context.Context,
	data []byte,
) (orphanFixHeaderParts, bool, error) {
	pos := 0

	for pos <= len(data) {
		if err := ctx.Err(); err != nil {
			return orphanFixHeaderParts{}, false, err
		}

		line, rest, found := bytes.Cut(data[pos:], []byte("\n"))
		lineForCheck := orphanFixTrimTrailingCR(line)

		if !checks.IsBlankUnicode(lineForCheck) {
			headerEnd := len(data) - len(rest)
			if !found {
				headerEnd = len(data)
			}

			return orphanFixHeaderParts{
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

	return orphanFixHeaderParts{}, false, nil
}

func orphanFixTrimTrailingCR(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}

	return line
}

func appendOrphanFixHeaderAndRest(parts orphanFixHeaderParts) []byte {
	out := make([]byte, 0, len(parts.line)+len(parts.rest))
	out = append(out, parts.line...)
	out = append(out, parts.rest...)

	return out
}

func readOrphanFixRecords(ctx context.Context, data []byte) ([][]string, error) {
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

func writeOrphanFixRecords(
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

type orphanFixColumn struct {
	label  string
	srcIdx int
}

type orphanFixPlan struct {
	columns       []orphanFixColumn
	insertedBases []string
}

func (p orphanFixPlan) hasChanges() bool {
	return len(p.insertedBases) > 0
}

func buildOrphanFixPlan(header []string) orphanFixPlan {
	originalColumns := make(map[string]struct{}, len(header))

	for _, col := range header {
		name := orphanFixNormalizeHeaderCell(col)
		if name == "" {
			continue
		}

		originalColumns[name] = struct{}{}
	}

	addedBases := make(map[string]struct{})
	plan := orphanFixPlan{
		columns:       make([]orphanFixColumn, 0, len(header)),
		insertedBases: make([]string, 0),
	}

	for idx, col := range header {
		label := strings.TrimSpace(col)
		name := orphanFixNormalizeHeaderCell(col)

		if base, ok := orphanFixDescriptionBase(name); ok {
			if _, exists := originalColumns[base]; !exists {
				if _, alreadyAdded := addedBases[base]; !alreadyAdded {
					plan.columns = append(plan.columns, orphanFixColumn{
						label:  base,
						srcIdx: -1,
					})
					plan.insertedBases = append(plan.insertedBases, base)
					addedBases[base] = struct{}{}
				}
			}
		}

		plan.columns = append(plan.columns, orphanFixColumn{
			label:  label,
			srcIdx: idx,
		})
	}

	return plan
}

func orphanFixNormalizeHeaderCell(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func orphanFixDescriptionBase(name string) (string, bool) {
	if !strings.HasSuffix(name, "_description") {
		return "", false
	}

	base := strings.TrimSuffix(name, "_description")
	base = strings.TrimSpace(base)

	if base == "" {
		return "", false
	}

	return base, true
}

func applyOrphanFixPlan(
	ctx context.Context,
	records [][]string,
	plan orphanFixPlan,
) ([][]string, error) {
	out := make([][]string, len(records))

	for i := range len(records) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		row := records[i]
		newRow := make([]string, len(plan.columns))

		for j, col := range plan.columns {
			if i == 0 {
				newRow[j] = col.label
				continue
			}

			if col.srcIdx >= 0 && col.srcIdx < len(row) {
				newRow[j] = row[col.srcIdx]
			}
		}

		out[i] = newRow
	}

	return out, nil
}

func orphanFixIsBlankCSVRecord(record []string) bool {
	for _, field := range record {
		if !checks.IsBlankUnicode([]byte(field)) {
			return false
		}
	}

	return true
}

func stitchOrphanFix(bom, before, tail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(tail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, tail...)

	return out
}
