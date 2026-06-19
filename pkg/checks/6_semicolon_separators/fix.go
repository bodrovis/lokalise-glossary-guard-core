package semicolon_separator

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixToSemicolonsIfConsistent(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in, bom := checks.SplitUTF8BOM(a.Data)
	if checks.IsBlankUnicode(in) {
		return noSeparatorFix(a, "no usable content to convert"), checks.ErrNoFix
	}

	lineSep := checks.DetectLineEnding(in)
	keepFinalNewline := hasFinalNewline(in)

	alreadyOK, err := attemptRectParse(ctx, in, ';')
	if err != nil {
		return checks.FixResult{}, err
	}
	if alreadyOK {
		return noSeparatorFix(a, "already semicolon-separated"), nil
	}

	alt, ok, err := detectConvertibleDelimiter(ctx, in)
	if err != nil {
		return checks.FixResult{}, err
	}
	if !ok {
		return noSeparatorFix(a, "cannot confidently detect delimiter; skipped auto-convert"), checks.ErrNoFix
	}

	converted := writeCSVWithSep(alt.records, ';', lineSep, keepFinalNewline)
	converted = prependBOM(bom, converted)

	return checks.FixResult{
		Data:      converted,
		Path:      "",
		DidChange: true,
		Note:      fmt.Sprintf("converted from %s to semicolons", alt.name),
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

func detectConvertibleDelimiter(ctx context.Context, data []byte) (convertibleDelimiter, bool, error) {
	commaRecs, commaOK, err := parseRectRecords(ctx, data, ',')
	if err != nil {
		return convertibleDelimiter{}, false, err
	}

	tabRecs, tabOK, err := parseRectRecords(ctx, data, '\t')
	if err != nil {
		return convertibleDelimiter{}, false, err
	}

	switch {
	case commaOK && !tabOK:
		return convertibleDelimiter{name: "commas", records: commaRecs}, true, nil
	case tabOK && !commaOK:
		return convertibleDelimiter{name: "tabs", records: tabRecs}, true, nil
	default:
		// none detected OR ambiguous comma+tab detection
		return convertibleDelimiter{}, false, nil
	}
}

func noSeparatorFix(a checks.Artifact, note string) checks.FixResult {
	return checks.FixResult{
		Data:      a.Data,
		Path:      "",
		DidChange: false,
		Note:      note,
	}
}

func writeCSVWithSep(recs [][]string, delim rune, lineSep string, keepFinal bool) []byte {
	var b strings.Builder

	for i, row := range recs {
		for j, col := range row {
			if j > 0 {
				b.WriteRune(delim)
			}
			b.WriteString(escapeCSVField(col, delim))
		}

		if i < len(recs)-1 || keepFinal {
			b.WriteString(lineSep)
		}
	}

	return []byte(b.String())
}

func escapeCSVField(field string, delim rune) string {
	if !needsCSVQuotes(field, delim) {
		return field
	}

	return `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
}

func needsCSVQuotes(field string, delim rune) bool {
	return strings.ContainsRune(field, delim) ||
		strings.ContainsAny(field, "\"\n\r")
}

func hasFinalNewline(b []byte) bool {
	return bytes.HasSuffix(b, []byte("\n"))
}
