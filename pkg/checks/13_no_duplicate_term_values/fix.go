package duplicate_term_values

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixDuplicateTermValues(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		return checks.NoFix(a, "no usable content to fix")
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	parts, ok, err := findDuplicateTermHeaderLine(ctx, in)
	if err != nil {
		return checks.FixResult{}, err
	}
	if !ok {
		return checks.NoFix(a, "no header with 'term' column found")
	}

	records, err := readDuplicateTermRecords(ctx, appendDuplicateTermHeaderAndRest(parts))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return checks.FixResult{}, ctxErr
		}

		return checks.NoFix(a, "cannot parse CSV with semicolon delimiter")
	}
	if len(records) == 0 || len(records[0]) == 0 {
		return checks.NoFix(a, "no header with 'term' column found")
	}

	termCol := findTermColumn(records[0])
	if termCol < 0 {
		return checks.NoFix(a, "no 'term' column found")
	}

	plan := buildDuplicateTermFixPlan(records, termCol, parts.headerLineNo)
	if !plan.hasDuplicates() {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no duplicate term rows to remove",
		}, nil
	}

	outTail, err := writeDuplicateTermRecords(ctx, plan.records, lineSep, keepFinal)
	if err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "failed to serialize CSV: " + err.Error(),
		}, err
	}

	out := stitchDuplicateTermFix(bom, parts.before, outTail)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      duplicateTermFixNote(plan.removed),
	}, nil
}

type duplicateTermHeaderParts struct {
	before       []byte
	line         []byte
	rest         []byte
	headerLineNo int
}

func findDuplicateTermHeaderLine(
	ctx context.Context,
	data []byte,
) (duplicateTermHeaderParts, bool, error) {
	pos := 0

	for pos <= len(data) {
		if err := ctx.Err(); err != nil {
			return duplicateTermHeaderParts{}, false, err
		}

		line, rest, found := bytes.Cut(data[pos:], []byte("\n"))
		lineForCheck := trimTrailingCR(line)

		if !checks.IsBlankUnicode(lineForCheck) {
			headerEnd := len(data) - len(rest)
			if !found {
				headerEnd = len(data)
			}

			return duplicateTermHeaderParts{
				before:       data[:pos],
				line:         data[pos:headerEnd],
				rest:         data[headerEnd:],
				headerLineNo: 1 + bytes.Count(data[:pos], []byte("\n")),
			}, true, nil
		}

		if !found {
			break
		}

		pos += len(line) + 1
	}

	return duplicateTermHeaderParts{}, false, nil
}

func appendDuplicateTermHeaderAndRest(parts duplicateTermHeaderParts) []byte {
	out := make([]byte, 0, len(parts.line)+len(parts.rest))
	out = append(out, parts.line...)
	out = append(out, parts.rest...)

	return out
}

func trimTrailingCR(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}

	return line
}

func readDuplicateTermRecords(ctx context.Context, data []byte) ([][]string, error) {
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

func writeDuplicateTermRecords(
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

type duplicateTermFixPlan struct {
	records []stringSlice
	removed []removedDuplicateTerm
}

type stringSlice = []string

type removedDuplicateTerm struct {
	term string
	rows []int
}

func (p duplicateTermFixPlan) hasDuplicates() bool {
	return len(p.removed) > 0
}

func buildDuplicateTermFixPlan(
	records [][]string,
	termCol int,
	headerLineNo int,
) duplicateTermFixPlan {
	seen := make(map[string]struct{})
	removedByTerm := make(map[string]*removedDuplicateTerm)
	removedOrder := make([]string, 0)

	out := make([][]string, 0, len(records))
	out = append(out, records[0])

	for i := 1; i < len(records); i++ {
		rec := records[i]

		term, ok := duplicateTermValue(rec, termCol)
		if !ok {
			out = append(out, rec)
			continue
		}

		if _, exists := seen[term]; !exists {
			seen[term] = struct{}{}
			out = append(out, rec)
			continue
		}

		info := removedByTerm[term]
		if info == nil {
			info = &removedDuplicateTerm{term: term}
			removedByTerm[term] = info
			removedOrder = append(removedOrder, term)
		}

		info.rows = append(info.rows, headerLineNo+i)
	}

	removed := make([]removedDuplicateTerm, 0, len(removedOrder))
	for _, term := range removedOrder {
		removed = append(removed, *removedByTerm[term])
	}

	return duplicateTermFixPlan{
		records: out,
		removed: removed,
	}
}

func duplicateTermValue(record []string, termCol int) (string, bool) {
	if termCol >= len(record) {
		return "", false
	}

	term := strings.TrimSpace(record[termCol])
	if term == "" {
		return "", false
	}

	return term, true
}

func duplicateTermFixNote(removed []removedDuplicateTerm) string {
	var b strings.Builder
	b.WriteString("removed duplicate term rows for: ")

	for i, info := range removed {
		if i > 0 {
			b.WriteString("; ")
		}

		b.WriteString(strconv.Quote(info.term))
		b.WriteString(" (rows ")
		b.WriteString(joinInts(info.rows, ", "))
		b.WriteString(")")
	}

	return b.String()
}

func joinInts(nums []int, sep string) string {
	if len(nums) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(strconv.Itoa(nums[0]))

	for _, n := range nums[1:] {
		b.WriteString(sep)
		b.WriteString(strconv.Itoa(n))
	}

	return b.String()
}

func stitchDuplicateTermFix(bom, before, tail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(tail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, tail...)

	return out
}
