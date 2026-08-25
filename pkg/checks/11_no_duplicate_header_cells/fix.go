package duplicate_header_cells

import (
	"bytes"
	"context"
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

	parts, ok, err := checks.FindFirstNonBlankPhysicalLine(ctx, in)
	if err != nil {
		return checks.FixResult{}, err
	}

	if !ok {
		return checks.NoFix(a, "no header line found")
	}

	before := parts.Before
	after := in[len(parts.Before):]

	records, err := readDuplicateHeaderRecords(ctx, after)
	if err != nil {
		return checks.FixResult{}, err
	}
	if len(records) == 0 || len(records[0]) == 0 {
		return checks.NoFix(a, "cannot parse CSV with semicolon delimiter")
	}

	plan := buildDuplicateHeaderPlan(records[0])
	if !plan.hasDuplicates() {
		return checks.NoChange(
			a,
			"no duplicate header columns to remove",
		), nil
	}

	outRecs, err := applyDuplicateHeaderPlan(ctx, records, plan)
	if err != nil {
		return checks.FixResult{}, err
	}

	outTail, err := checks.WriteSemicolonCSVRecords(
		ctx,
		outRecs,
		lineSep,
		keepFinal,
	)
	if err != nil {
		return checks.NoChange(
			a,
			"failed to serialize CSV: "+err.Error(),
		), err
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

func readDuplicateHeaderRecords(
	ctx context.Context,
	data []byte,
) ([][]string, error) {
	r := checks.NewSemicolonCSVReader(data)

	return checks.ReadAllCSVRecords(ctx, r)
}

func buildDuplicateHeaderPlan(header []string) duplicateHeaderPlan {
	seen := make(map[string]struct{}, len(header))

	plan := duplicateHeaderPlan{
		keepIdx:      make([]int, 0, len(header)),
		removedNames: make([]string, 0),
	}

	for i, col := range header {
		key := checks.NormalizeStr(col)
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

func stitchDuplicateHeaderFix(bom, before, tail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(tail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, tail...)

	return out
}
