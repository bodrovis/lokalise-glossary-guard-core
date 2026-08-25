package term_description_header

import (
	"bytes"
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

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
	parts     checks.PhysicalLineParts
	records   [][]string
}

func fixTermDescriptionHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	source, noFixNote, err := prepareTermDescriptionFix(ctx, a)
	if err != nil {
		return checks.FixResult{}, err
	}

	if noFixNote != "" {
		return checks.NoFix(a, noFixNote)
	}

	plan := buildTermDescriptionPlan(source.records[0])
	if plan.alreadyOK {
		return checks.NoChange(
			a,
			"header already starts with term;description",
		), nil
	}

	outRecs, err := applyTermDescriptionPlan(ctx, source.records, plan)
	if err != nil {
		return checks.FixResult{}, err
	}

	body, err := serializeTermDescriptionRecords(ctx, outRecs, source)
	if err != nil {
		return checks.NoChange(
			a,
			"failed to serialize CSV: "+err.Error(),
		), err
	}

	return checks.FixResult{
		Data:      stitchFixedCSV(source.bom, source.parts.Before, body),
		Path:      "",
		DidChange: true,
		Note:      termDescriptionFixNote(plan),
	}, nil
}

func prepareTermDescriptionFix(
	ctx context.Context,
	a checks.Artifact,
) (termDescriptionFixSource, string, error) {
	in, bom := checks.SplitUTF8BOM(a.Data)

	if checks.IsBlankUnicode(in) {
		return termDescriptionFixSource{},
			"no usable content to fix",
			nil
	}

	parts, ok, err := checks.FindFirstNonBlankPhysicalLine(ctx, in)
	if err != nil {
		return termDescriptionFixSource{}, "", err
	}

	if !ok {
		return termDescriptionFixSource{},
			"no header line found",
			nil
	}

	records, ok, err := readTermDescriptionRecords(ctx, parts)
	if err != nil {
		return termDescriptionFixSource{}, "", err
	}

	if !ok {
		return termDescriptionFixSource{},
			"cannot parse CSV with semicolon delimiter",
			nil
	}

	return termDescriptionFixSource{
		bom:       bom,
		lineSep:   checks.DetectLineEnding(in),
		keepFinal: bytes.HasSuffix(in, []byte("\n")),
		parts:     parts,
		records:   records,
	}, "", nil
}

func readTermDescriptionRecords(
	ctx context.Context,
	parts checks.PhysicalLineParts,
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

func appendHeaderAndRest(
	parts checks.PhysicalLineParts,
) []byte {
	data := make([]byte, 0, len(parts.Line)+len(parts.Rest))
	data = append(data, parts.Line...)
	data = append(data, parts.Rest...)

	return data
}

func serializeTermDescriptionRecords(
	ctx context.Context,
	records [][]string,
	source termDescriptionFixSource,
) ([]byte, error) {
	return checks.WriteSemicolonCSVRecords(
		ctx,
		records,
		source.lineSep,
		source.keepFinal,
	)
}

func termDescriptionFixNote(plan termDescriptionPlan) string {
	if !plan.hasTerm || !plan.hasDescription {
		return "inserted missing term/description columns at start"
	}

	return "reordered columns to start with term;description"
}

func buildTermDescriptionPlan(header []string) termDescriptionPlan {
	plan := termDescriptionPlan{
		termIndex: -1,
		descIndex: -1,
	}

	for i, col := range header {
		switch checks.NormalizeStr(col) {
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
		checks.NormalizeStr(header[0]) == "term" &&
		checks.NormalizeStr(header[1]) == "description"

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

func readSemicolonRecords(
	ctx context.Context,
	data []byte,
) ([][]string, error) {
	r := checks.NewSemicolonCSVReader(data)

	return checks.ReadAllCSVRecords(ctx, r)
}

func stitchFixedCSV(bom, before, body []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(body))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, body...)

	return out
}
