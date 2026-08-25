package duplicate_term_values

import (
	"bytes"
	"context"
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

	parts, ok, err := checks.FindFirstNonBlankPhysicalLine(ctx, in)
	if err != nil {
		return checks.FixResult{}, err
	}

	if !ok {
		return checks.NoFix(a, "no header with 'term' column found")
	}

	headerLineNo := 1 + bytes.Count(parts.Before, []byte("\n"))

	records, err := readDuplicateTermRecords(
		ctx,
		appendDuplicateTermHeaderAndRest(parts),
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return checks.FixResult{}, ctxErr
		}

		return checks.NoFix(a, "cannot parse CSV with semicolon delimiter")
	}
	if len(records) == 0 || len(records[0]) == 0 {
		return checks.NoFix(a, "no header with 'term' column found")
	}

	termCol := checks.FindHeaderColumn(records[0], "term")
	if termCol < 0 {
		return checks.NoFix(a, "no 'term' column found")
	}

	plan := buildDuplicateTermFixPlan(records, termCol, headerLineNo)
	if !plan.hasDuplicates() {
		return checks.NoChange(
			a,
			"no duplicate term rows to remove",
		), nil
	}

	outTail, err := checks.WriteSemicolonCSVRecords(
		ctx,
		plan.records,
		lineSep,
		keepFinal,
	)
	if err != nil {
		return checks.NoChange(
			a,
			"failed to serialize CSV: "+err.Error(),
		), err
	}

	out := stitchDuplicateTermFix(bom, parts.Before, outTail)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      duplicateTermFixNote(plan.removed),
	}, nil
}

func appendDuplicateTermHeaderAndRest(
	parts checks.PhysicalLineParts,
) []byte {
	out := make([]byte, 0, len(parts.Line)+len(parts.Rest))
	out = append(out, parts.Line...)
	out = append(out, parts.Rest...)

	return out
}

func readDuplicateTermRecords(
	ctx context.Context,
	data []byte,
) ([][]string, error) {
	r := checks.NewSemicolonCSVReader(data)

	return checks.ReadAllCSVRecords(ctx, r)
}

type duplicateTermFixPlan struct {
	records [][]string
	removed []removedDuplicateTerm
}

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

		term, ok := termValue(rec, termCol)
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

func duplicateTermFixNote(removed []removedDuplicateTerm) string {
	var b strings.Builder
	b.WriteString("removed duplicate term rows for: ")

	for i, info := range removed {
		if i > 0 {
			b.WriteString("; ")
		}

		b.WriteString(strconv.Quote(info.term))
		b.WriteString(" (rows ")
		b.WriteString(joinIntSlice(info.rows, ", "))
		b.WriteString(")")
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
