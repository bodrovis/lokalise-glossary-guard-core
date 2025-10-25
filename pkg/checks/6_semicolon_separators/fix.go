package semicolon_separator

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixToSemicolonsIfConsistent(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	data := strings.TrimSpace(string(a.Data))
	if data == "" {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no usable content to convert",
		}, checks.ErrNoFix
	}

	semiOK, _ := attemptRectParse(data, ';')
	if semiOK {
		// уже ок
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "already semicolon-separated",
		}, nil
	}

	commaOK, commaRecs := attemptRectParse(data, ',')
	tabOK, tabRecs := attemptRectParse(data, '\t')

	switch {
	case tabOK:
		out, err := writeCSV(tabRecs, ';')
		if err != nil {
			return checks.FixResult{
				Data:      a.Data,
				Path:      "",
				DidChange: false,
				Note:      "",
			}, err
		}
		return checks.FixResult{
			Data:      out,
			Path:      "",
			DidChange: true,
			Note:      "converted from tabs to semicolons",
		}, nil

	case commaOK:
		out, err := writeCSV(commaRecs, ';')
		if err != nil {
			return checks.FixResult{
				Data:      a.Data,
				Path:      "",
				DidChange: false,
				Note:      "",
			}, err
		}
		return checks.FixResult{
			Data:      out,
			Path:      "",
			DidChange: true,
			Note:      "converted from commas to semicolons",
		}, nil
	}

	// ни один формат не дал прямоугольность => не трогаем
	return checks.FixResult{
		Data:      a.Data,
		Path:      "",
		DidChange: false,
		Note:      "cannot confidently detect delimiter; skipped auto-convert",
	}, checks.ErrNoFix
}

// writeCSV serializes records with the given delimiter (target ';').
func writeCSV(recs [][]string, delim rune) ([]byte, error) {
	var b strings.Builder

	escape := func(field string) string {
		needsQuote := strings.ContainsRune(field, delim) ||
			strings.ContainsAny(field, "\"\n\r")
		if !needsQuote {
			return field
		}

		return `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}

	for rowIdx, row := range recs {
		for colIdx, col := range row {
			if colIdx > 0 {
				b.WriteRune(delim)
			}
			b.WriteString(escape(col))
		}

		if rowIdx != len(recs)-1 {
			b.WriteByte('\n')
		}
	}

	return []byte(b.String()), nil
}
