package allowed_columns_header

import (
	"bytes"
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixAllowedColumnsHeader(
	ctx context.Context,
	a checks.Artifact,
) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	source, noFixNote, err := prepareAllowedColumnsFix(ctx, a)
	if err != nil {
		return checks.FixResult{}, err
	}

	if noFixNote != "" {
		return checks.NoFix(a, noFixNote)
	}

	plan := buildAllowedColumnsPlan(source.header(), a.Langs)

	if plan.isNoOp(source.header()) {
		return checks.NoChange(
			a,
			"header already normalized",
		), nil
	}

	outRecs, err := applyAllowedColumnsPlan(ctx, source.records, plan)
	if err != nil {
		return checks.FixResult{}, err
	}

	outTail, err := checks.WriteSemicolonCSVRecords(
		ctx,
		outRecs,
		source.lineSep,
		source.keepFinal,
	)
	if err != nil {
		return checks.NoChange(
			a,
			"failed to serialize CSV: "+err.Error(),
		), err
	}

	out := stitchAllowedColumnsFix(
		source.bom,
		source.before,
		outTail,
	)

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

func prepareAllowedColumnsFix(
	ctx context.Context,
	a checks.Artifact,
) (allowedColumnsFixSource, string, error) {
	in, bom := checks.SplitUTF8BOM(a.Data)

	if checks.IsBlankUnicode(in) {
		return allowedColumnsFixSource{},
			"no usable content to fix header",
			nil
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	parts, ok, err := checks.FindFirstNonBlankPhysicalLine(ctx, in)
	if err != nil {
		return allowedColumnsFixSource{}, "", err
	}

	if !ok {
		return allowedColumnsFixSource{},
			"no header line found",
			nil
	}

	tail := appendHeaderAndRest(parts.Line, parts.Rest)

	records, err := readAllowedColumnsRecords(ctx, tail)
	if err != nil {
		return allowedColumnsFixSource{}, "", err
	}

	if len(records) == 0 || len(records[0]) == 0 {
		return allowedColumnsFixSource{},
			"cannot parse CSV with semicolon delimiter",
			nil
	}

	return allowedColumnsFixSource{
		bom:       bom,
		before:    parts.Before,
		lineSep:   lineSep,
		keepFinal: keepFinal,
		records:   records,
	}, "", nil
}

func appendHeaderAndRest(header, rest []byte) []byte {
	out := make([]byte, 0, len(header)+len(rest))
	out = append(out, header...)
	out = append(out, rest...)

	return out
}

func readAllowedColumnsRecords(
	ctx context.Context,
	data []byte,
) ([][]string, error) {
	r := checks.NewSemicolonCSVReader(data)

	return checks.ReadAllCSVRecords(ctx, r)
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
		if checks.NormalizeStr(p.keep[i].label) != checks.NormalizeStr(header[i]) {
			return false
		}

		if p.keep[i].idx != i {
			return false
		}
	}

	return true
}

type langPresence struct {
	base  bool
	desc  bool
	label string
}

func allowedColumnFromHeader(
	name string,
	idx int,
	declared declaredLanguages,
	seen map[string]langPresence,
) (allowedColumn, bool) {
	normalized := checks.NormalizeStr(name)

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

	if seenEntry.label == "" {
		seenEntry.label = langCol.base
	}

	if langCol.description {
		seenEntry.desc = true
	} else {
		seenEntry.base = true
	}

	seen[langCol.key] = seenEntry

	return allowedColumn{
		label: strings.TrimSpace(name),
		idx:   idx,
	}, true
}

func (p *allowedColumnsPlan) addMissingDeclaredLanguages(
	declared declaredLanguages,
	seen map[string]langPresence,
) {
	for _, lang := range declared.order {
		presence := seen[lang]

		label := presence.label
		if label == "" {
			label = declared.labels[lang]
		}

		if !presence.base {
			p.keep = append(p.keep, allowedColumn{
				label: label,
				idx:   -1,
			})
		}

		if !presence.desc {
			p.keep = append(p.keep, allowedColumn{
				label: label + "_description",
				idx:   -1,
			})
		}
	}
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

func stitchAllowedColumnsFix(bom, before, outTail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(outTail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, outTail...)

	return out
}
