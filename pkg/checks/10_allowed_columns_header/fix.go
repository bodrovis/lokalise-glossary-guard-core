package allowed_columns_header

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixAllowedColumnsHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	source, early, err := prepareAllowedColumnsFix(ctx, a)
	if err != nil {
		return checks.FixResult{}, err
	}
	if early != nil {
		return early.result, early.err
	}

	plan := buildAllowedColumnsPlan(source.header(), a.Langs)
	if plan.isNoOp(source.header()) {
		return checks.FixResult{
			Data:      a.Data,
			Path:      a.Path,
			DidChange: false,
			Note:      "header already normalized",
		}, nil
	}

	outRecs, err := applyAllowedColumnsPlan(ctx, source.records, plan)
	if err != nil {
		return checks.FixResult{}, err
	}

	outTail, err := serializeAllowedColumnsRecords(ctx, outRecs, source.lineSep, source.keepFinal)
	if err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      a.Path,
			DidChange: false,
			Note:      "failed to serialize CSV: " + err.Error(),
		}, err
	}

	out := stitchAllowedColumnsFix(source.bom, source.before, outTail)

	return checks.FixResult{
		Data:      out,
		Path:      a.Path,
		DidChange: true,
		Note:      "removed unknown columns and ensured declared languages are present",
	}, nil
}

type allowedColumnsFixSource struct {
	bom       []byte
	before    []byte
	lineSep   string
	keepFinal bool
	records   [][]string
}

func (s allowedColumnsFixSource) header() []string {
	if len(s.records) == 0 {
		return nil
	}

	return s.records[0]
}

type earlyAllowedColumnsFix struct {
	result checks.FixResult
	err    error
}

func prepareAllowedColumnsFix(
	ctx context.Context,
	a checks.Artifact,
) (allowedColumnsFixSource, *earlyAllowedColumnsFix, error) {
	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		return allowedColumnsFixSource{}, noAllowedColumnsFixEarly(a, "no usable content to fix header"), nil
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	parts, ok, err := findAllowedColumnsHeaderLine(ctx, in)
	if err != nil {
		return allowedColumnsFixSource{}, nil, err
	}
	if !ok {
		return allowedColumnsFixSource{}, noAllowedColumnsFixEarly(a, "no header line found"), nil
	}

	tail := appendHeaderAndRest(parts.line, parts.rest)

	records, err := readAllowedColumnsRecords(ctx, tail)
	if err != nil {
		return allowedColumnsFixSource{}, nil, err
	}
	if len(records) == 0 || len(records[0]) == 0 {
		return allowedColumnsFixSource{}, noAllowedColumnsFixEarly(a, "cannot parse CSV with semicolon delimiter"), nil
	}

	return allowedColumnsFixSource{
		bom:       bom,
		before:    parts.before,
		lineSep:   lineSep,
		keepFinal: keepFinal,
		records:   records,
	}, nil, nil
}

func noAllowedColumnsFixEarly(a checks.Artifact, note string) *earlyAllowedColumnsFix {
	result, err := checks.NoFix(a, note)

	return &earlyAllowedColumnsFix{
		result: result,
		err:    err,
	}
}

type allowedColumnsHeaderParts struct {
	before []byte
	line   []byte
	rest   []byte
}

func findAllowedColumnsHeaderLine(
	ctx context.Context,
	data []byte,
) (allowedColumnsHeaderParts, bool, error) {
	pos := 0

	for pos <= len(data) {
		if err := ctx.Err(); err != nil {
			return allowedColumnsHeaderParts{}, false, err
		}

		line, rest, found := bytes.Cut(data[pos:], []byte("\n"))
		lineForCheck := trimTrailingCR(line)

		if !checks.IsBlankUnicode(lineForCheck) {
			headerEnd := len(data) - len(rest)
			if !found {
				headerEnd = len(data)
			}

			return allowedColumnsHeaderParts{
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

	return allowedColumnsHeaderParts{}, false, nil
}

func trimTrailingCR(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}

	return line
}

func appendHeaderAndRest(header, rest []byte) []byte {
	out := make([]byte, 0, len(header)+len(rest))
	out = append(out, header...)
	out = append(out, rest...)

	return out
}

func readAllowedColumnsRecords(ctx context.Context, data []byte) ([][]string, error) {
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

type allowedColumnsPlan struct {
	keep []allowedColumn
}

type allowedColumn struct {
	label string
	idx   int
}

func buildAllowedColumnsPlan(header []string, langs []string) allowedColumnsPlan {
	declared := newDeclaredLanguages(langs)

	plan := allowedColumnsPlan{
		keep: make([]allowedColumn, 0, len(header)+len(declared.order)*2),
	}

	seenLang := make(map[string]langPresence, len(declared.order))

	for idx, name := range header {
		col, ok := allowedColumnFromHeader(name, idx, declared, seenLang)
		if !ok {
			continue
		}

		plan.keep = append(plan.keep, col)
	}

	if declared.hasAny() {
		plan.addMissingDeclaredLanguages(declared, seenLang)
	}

	return plan
}

func (p allowedColumnsPlan) isNoOp(header []string) bool {
	if len(p.keep) != len(header) {
		return false
	}

	for i := range p.keep {
		if normalizeHeaderName(p.keep[i].label) != normalizeHeaderName(header[i]) {
			return false
		}

		if p.keep[i].idx != i {
			return false
		}
	}

	return true
}

type declaredLanguages struct {
	order []string
	set   map[string]struct{}
}

func newDeclaredLanguages(langs []string) declaredLanguages {
	out := declaredLanguages{
		set: make(map[string]struct{}, len(langs)),
	}

	for _, lang := range langs {
		key := normalizeLangKey(lang)
		if key == "" {
			continue
		}

		if _, exists := out.set[key]; exists {
			continue
		}

		out.set[key] = struct{}{}
		out.order = append(out.order, key)
	}

	return out
}

func (d declaredLanguages) hasAny() bool {
	return len(d.order) > 0
}

func (d declaredLanguages) contains(lang string) bool {
	_, ok := d.set[normalizeLangKey(lang)]
	return ok
}

type langPresence struct {
	base bool
	desc bool
}

func allowedColumnFromHeader(
	name string,
	idx int,
	declared declaredLanguages,
	seen map[string]langPresence,
) (allowedColumn, bool) {
	normalized := normalizeHeaderName(name)

	if _, ok := checks.KnownHeaders[normalized]; ok {
		return allowedColumn{label: name, idx: idx}, true
	}

	langCol, isLang := parseLangColumn(name)
	if !isLang {
		return allowedColumn{}, false
	}

	if declared.hasAny() && !declared.contains(langCol.key) {
		return allowedColumn{}, false
	}

	seenEntry := seen[langCol.key]
	if langCol.description {
		seenEntry.desc = true
	} else {
		seenEntry.base = true
	}
	seen[langCol.key] = seenEntry

	return allowedColumn{
		label: normalizedLangColumnLabel(langCol),
		idx:   idx,
	}, true
}

func normalizedLangColumnLabel(col parsedLangColumn) string {
	if col.description {
		return col.key + "_description"
	}

	return col.key
}

func (p *allowedColumnsPlan) addMissingDeclaredLanguages(
	declared declaredLanguages,
	seen map[string]langPresence,
) {
	for _, lang := range declared.order {
		presence := seen[lang]

		if !presence.base {
			p.keep = append(p.keep, allowedColumn{
				label: lang,
				idx:   -1,
			})
		}

		if !presence.desc {
			p.keep = append(p.keep, allowedColumn{
				label: lang + "_description",
				idx:   -1,
			})
		}
	}
}

func normalizeHeaderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func applyAllowedColumnsPlan(
	ctx context.Context,
	records [][]string,
	plan allowedColumnsPlan,
) ([][]string, error) {
	out := make([][]string, len(records))

	for i, row := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		newRow := make([]string, len(plan.keep))

		for j, col := range plan.keep {
			if col.idx >= 0 && col.idx < len(row) {
				newRow[j] = row[col.idx]
			}
		}

		out[i] = newRow
	}

	out[0] = plan.headerLabels()

	return out, nil
}

func (p allowedColumnsPlan) headerLabels() []string {
	header := make([]string, len(p.keep))

	for i, col := range p.keep {
		header[i] = col.label
	}

	return header
}

func serializeAllowedColumnsRecords(
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

func stitchAllowedColumnsFix(bom, before, outTail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(outTail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, outTail...)

	return out
}
