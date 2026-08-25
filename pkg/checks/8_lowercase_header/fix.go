package lowercase_header

import (
	"bytes"
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixLowercaseHeader(
	ctx context.Context,
	a checks.Artifact,
) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)

	if checks.IsBlankUnicode(in) {
		return checks.NoFix(a, "no usable content to normalize header")
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

	record, err := readHeaderRecordForFix(parts.Line)
	if err != nil || len(record) == 0 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return checks.FixResult{}, ctxErr
		}

		return checks.NoFix(
			a,
			"cannot parse header with semicolon delimiter",
		)
	}

	changed, err := lowercaseKnownHeaderColumns(ctx, record)
	if err != nil {
		return checks.FixResult{}, err
	}

	if !changed {
		return checks.NoChange(
			a,
			"header service columns already lowercase",
		), nil
	}

	keepHeaderFinal := keepFinal || len(parts.Rest) > 0

	newHeader, err := checks.WriteSemicolonCSVRecords(
		ctx,
		[][]string{record},
		lineSep,
		keepHeaderFinal,
	)
	if err != nil {
		return checks.NoChange(
			a,
			"failed to serialize normalized header",
		), err
	}

	out := stitchHeaderFix(
		bom,
		parts.Before,
		newHeader,
		parts.Rest,
	)

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "normalized service columns in header to lowercase",
	}, nil
}

func readHeaderRecordForFix(headerLine []byte) ([]string, error) {
	r := checks.NewSemicolonCSVReader(headerLine)

	return r.Read()
}

func lowercaseKnownHeaderColumns(
	ctx context.Context,
	record []string,
) (bool, error) {
	changed := false

	for i, col := range record {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		lower, ok := lowercaseKnownHeaderColumn(col)
		if !ok {
			continue
		}

		if record[i] != lower {
			record[i] = lower
			changed = true
		}
	}

	return changed, nil
}

func lowercaseKnownHeaderColumn(col string) (string, bool) {
	lower := strings.ToLower(col)

	if _, ok := checks.KnownHeaders[lower]; !ok {
		return "", false
	}

	return lower, true
}

func stitchHeaderFix(
	bom []byte,
	before []byte,
	newHeader []byte,
	rest []byte,
) []byte {
	out := make(
		[]byte,
		0,
		len(bom)+len(before)+len(newHeader)+len(rest),
	)

	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, newHeader...)
	out = append(out, rest...)

	return out
}
