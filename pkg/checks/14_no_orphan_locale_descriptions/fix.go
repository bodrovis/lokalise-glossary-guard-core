package orphan_locale_descriptions

import (
	"bytes"
	"context"
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

	parts, ok, err := checks.FindFirstNonBlankPhysicalLine(ctx, in)
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
	if len(records) == 0 || checks.IsBlankCSVRecord(records[0]) {
		return checks.NoFix(a, "empty header line")
	}

	plan := buildOrphanFixPlan(records[0])
	if !plan.hasChanges() {
		return checks.NoChange(
			a,
			"no orphan *_description columns to fix",
		), nil
	}

	outRecs, err := applyOrphanFixPlan(ctx, records, plan)
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

	out := stitchOrphanFix(bom, parts.Before, outTail)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "added missing locale columns before *_description: " + strings.Join(plan.insertedBases, ", "),
	}, nil
}

func appendOrphanFixHeaderAndRest(
	parts checks.PhysicalLineParts,
) []byte {
	out := make([]byte, 0, len(parts.Line)+len(parts.Rest))
	out = append(out, parts.Line...)
	out = append(out, parts.Rest...)

	return out
}

func readOrphanFixRecords(
	ctx context.Context,
	data []byte,
) ([][]string, error) {
	r := checks.NewSemicolonCSVReader(data)

	return checks.ReadAllCSVRecords(ctx, r)
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
		name := checks.NormalizeStr(col)
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
		name := checks.NormalizeStr(col)

		if base, ok := descriptionBase(name); ok {
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

func stitchOrphanFix(bom, before, tail []byte) []byte {
	out := make([]byte, 0, len(bom)+len(before)+len(tail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, tail...)

	return out
}
