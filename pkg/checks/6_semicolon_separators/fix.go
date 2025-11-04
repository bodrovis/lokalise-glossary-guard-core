package semicolon_separator

import (
	"bytes"
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixToSemicolonsIfConsistent(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in := a.Data

	// Strip optional UTF-8 BOM, but remember to restore it if we actually convert.
	bom := []byte{}
	if bytes.HasPrefix(in, []byte{0xEF, 0xBB, 0xBF}) {
		bom = []byte{0xEF, 0xBB, 0xBF}
		in = in[3:]
	}

	// Treat files with only whitespace/zero-width as having no usable content.
	if checks.IsBlankUnicode(in) {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "no usable content to convert"}, checks.ErrNoFix
	}

	// detect line ending to preserve
	sep := checks.DetectLineEnding(in)
	dataStr := string(in)

	if ok, _ := attemptRectParse(dataStr, ';'); ok {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "already semicolon-separated"}, nil
	}

	commaOK, commaRecs := attemptRectParse(dataStr, ',')
	tabOK, tabRecs := attemptRectParse(dataStr, '\t')

	switch {
	case tabOK:
		out, err := writeCSVWithSep(tabRecs, ';', sep, hasFinalNewline(in))
		if err != nil {
			return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to convert from tabs: " + err.Error()}, err
		}
		return checks.FixResult{Data: append(bom, out...), Path: "", DidChange: true, Note: "converted from tabs to semicolons"}, nil

	case commaOK:
		out, err := writeCSVWithSep(commaRecs, ';', sep, hasFinalNewline(in))
		if err != nil {
			return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to convert from commas: " + err.Error()}, err
		}
		return checks.FixResult{Data: append(bom, out...), Path: "", DidChange: true, Note: "converted from commas to semicolons"}, nil
	}

	return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "cannot confidently detect delimiter; skipped auto-convert"}, checks.ErrNoFix
}

func writeCSVWithSep(recs [][]string, delim rune, lineSep string, keepFinal bool) ([]byte, error) {
	var b strings.Builder
	escape := func(field string) string {
		if strings.ContainsRune(field, delim) || strings.ContainsAny(field, "\"\n\r") {
			return `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
		}
		return field
	}
	for i, row := range recs {
		for j, col := range row {
			if j > 0 {
				b.WriteRune(delim)
			}
			b.WriteString(escape(col))
		}
		if i < len(recs)-1 || keepFinal {
			b.WriteString(lineSep)
		}
	}
	return []byte(b.String()), nil
}

func hasFinalNewline(b []byte) bool {
	return bytes.HasSuffix(b, []byte("\n"))
}
