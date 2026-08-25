package invalid_flags

import (
	"bytes"
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type flagFixInput struct {
	bom       []byte
	lineSep   string
	keepFinal bool
	parts     checks.PhysicalLineParts
}

func fixNoInvalidFlags(
	ctx context.Context,
	a checks.Artifact,
) (checks.FixResult, error) {
	prep, noFixNote, err := prepareFlagFixInput(ctx, a)
	if err != nil {
		return checks.FixResult{}, err
	}

	if noFixNote != "" {
		return checks.NoFix(a, noFixNote)
	}

	records, noFixNote, err := parseFlagFixRecords(ctx, prep)
	if err != nil {
		return checks.FixResult{}, err
	}

	if noFixNote != "" {
		return checks.NoFix(a, noFixNote)
	}

	outRecs, noChangeNote, err := buildFlagFixOutput(
		ctx,
		records,
	)
	if err != nil {
		return checks.FixResult{}, err
	}

	if noChangeNote != "" {
		return checks.NoChange(a, noChangeNote), nil
	}

	return serializeFlagFixResult(ctx, a, prep, outRecs)
}

func prepareFlagFixInput(
	ctx context.Context,
	a checks.Artifact,
) (flagFixInput, string, error) {
	if err := ctx.Err(); err != nil {
		return flagFixInput{}, "", err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)

	if checks.IsBlankUnicode(in) {
		return flagFixInput{},
			"no usable content to fix",
			nil
	}

	parts, ok, err := checks.FindFirstNonBlankPhysicalLine(
		ctx,
		in,
	)
	if err != nil {
		return flagFixInput{}, "", err
	}

	if !ok {
		return flagFixInput{},
			"no header line found",
			nil
	}

	return flagFixInput{
		bom:       bom,
		lineSep:   checks.DetectLineEnding(in),
		keepFinal: bytes.HasSuffix(in, []byte("\n")),
		parts:     parts,
	}, "", nil
}

func parseFlagFixRecords(
	ctx context.Context,
	prep flagFixInput,
) ([][]string, string, error) {
	records, err := readFlagFixRecords(
		ctx,
		appendFlagFixHeaderAndRest(prep.parts),
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", ctxErr
		}

		return nil,
			"cannot parse CSV with semicolon delimiter",
			nil
	}

	if len(records) == 0 ||
		checks.IsBlankCSVRecord(records[0]) {
		return nil,
			"empty header line",
			nil
	}

	return records, "", nil
}

func buildFlagFixOutput(
	ctx context.Context,
	records [][]string,
) ([][]string, string, error) {
	flagColumns := findFlagColumns(records[0])

	if len(flagColumns) == 0 {
		return nil,
			"no flag columns to normalize",
			nil
	}

	outRecs, changed, err := normalizeFlagRecords(
		ctx,
		records,
		flagColumns,
	)
	if err != nil {
		return nil, "", err
	}

	if !changed {
		return nil,
			"no flag values to normalize",
			nil
	}

	return outRecs, "", nil
}

func serializeFlagFixResult(
	ctx context.Context,
	a checks.Artifact,
	prep flagFixInput,
	outRecs [][]string,
) (checks.FixResult, error) {
	outTail, err := checks.WriteSemicolonCSVRecords(
		ctx,
		outRecs,
		prep.lineSep,
		prep.keepFinal,
	)
	if err != nil {
		return checks.NoChange(
			a,
			"failed to serialize CSV: "+err.Error(),
		), err
	}

	out := stitchFlagFix(prep.bom, prep.parts.Before, outTail)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "normalized flag columns to yes/no",
	}, nil
}

func appendFlagFixHeaderAndRest(
	parts checks.PhysicalLineParts,
) []byte {
	out := make([]byte, 0, len(parts.Line)+len(parts.Rest))
	out = append(out, parts.Line...)
	out = append(out, parts.Rest...)

	return out
}

func readFlagFixRecords(
	ctx context.Context,
	data []byte,
) ([][]string, error) {
	r := checks.NewSemicolonCSVReader(data)

	return checks.ReadAllCSVRecords(ctx, r)
}

func normalizeFlagRecords(
	ctx context.Context,
	records [][]string,
	flagColumns []flagColumn,
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

		if checks.IsBlankCSVRecord(row) {
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

func stitchFlagFix(bom, before, tail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(tail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, tail...)

	return out
}
