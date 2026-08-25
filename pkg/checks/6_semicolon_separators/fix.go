package semicolon_separator

import (
	"bytes"
	"context"
	"fmt"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixToSemicolonsIfConsistent(
	ctx context.Context,
	a checks.Artifact,
) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)

	if checks.IsBlankUnicode(in) {
		return checks.NoFix(
			a,
			"no usable content to convert",
		)
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinalNewline := bytes.HasSuffix(in, []byte("\n"))

	alreadyOK, err := attemptRectParse(ctx, in, ';')
	if err != nil {
		return checks.FixResult{}, err
	}

	if alreadyOK {
		return checks.NoChange(
			a,
			"already semicolon-separated",
		), nil
	}

	alt, ok, err := detectConvertibleDelimiter(ctx, in)
	if err != nil {
		return checks.FixResult{}, err
	}

	if !ok {
		return checks.NoFix(
			a,
			"cannot confidently detect delimiter; skipped auto-convert",
		)
	}

	converted, err := checks.WriteSemicolonCSVRecords(
		ctx,
		alt.records,
		lineSep,
		keepFinalNewline,
	)
	if err != nil {
		return checks.NoChange(
			a,
			"failed to serialize CSV: "+err.Error(),
		), err
	}

	converted = prependBOM(bom, converted)

	return checks.FixResult{
		Data:      converted,
		Path:      "",
		DidChange: true,
		Note: fmt.Sprintf(
			"converted from %s to semicolons",
			alt.name,
		),
	}, nil
}

func prependBOM(bom, data []byte) []byte {
	if len(bom) == 0 {
		return data
	}

	out := make([]byte, 0, len(bom)+len(data))
	out = append(out, bom...)
	out = append(out, data...)
	return out
}

type convertibleDelimiter struct {
	name    string
	records [][]string
}

type alternativeDelimiters struct {
	commaRecords [][]string
	commaOK      bool
	tabRecords   [][]string
	tabOK        bool
}

func detectAlternativeDelimiters(
	ctx context.Context,
	data []byte,
) (alternativeDelimiters, error) {
	commaRecords, commaOK, err := parseRectRecords(ctx, data, ',')
	if err != nil {
		return alternativeDelimiters{}, err
	}

	tabRecords, tabOK, err := parseRectRecords(ctx, data, '\t')
	if err != nil {
		return alternativeDelimiters{}, err
	}

	return alternativeDelimiters{
		commaRecords: commaRecords,
		commaOK:      commaOK,
		tabRecords:   tabRecords,
		tabOK:        tabOK,
	}, nil
}

func detectConvertibleDelimiter(
	ctx context.Context,
	data []byte,
) (convertibleDelimiter, bool, error) {
	alts, err := detectAlternativeDelimiters(ctx, data)
	if err != nil {
		return convertibleDelimiter{}, false, err
	}

	switch {
	case alts.commaOK && !alts.tabOK:
		return convertibleDelimiter{
			name:    "commas",
			records: alts.commaRecords,
		}, true, nil

	case alts.tabOK && !alts.commaOK:
		return convertibleDelimiter{
			name:    "tabs",
			records: alts.tabRecords,
		}, true, nil

	default:
		return convertibleDelimiter{}, false, nil
	}
}
