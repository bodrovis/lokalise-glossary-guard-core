package semicolon_separator

import (
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixToSemicolonsIfConsistent(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	data := strings.TrimSpace(string(a.Data))
	if data == "" {
		// Nothing to do; leave it to other checks.
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no usable content to convert",
		}, nil
	}

	const maxLines = 100

	// 1. Already semicolons? Nothing to change.
	if ok, _ := looksLikeDelimited(data, ';', maxLines); ok {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "already semicolon-separated",
		}, nil
	}

	// 2. Detect clean comma-separated or tab-separated input.
	//    If it's clean comma-separated, we SHOULD convert,
	//    even if there are semicolons inside quoted fields.
	commaOK, _ := looksLikeDelimited(data, ',', maxLines)
	tabOK, _ := looksLikeDelimited(data, '\t', maxLines)

	switch {
	case commaOK:
		// Safe to treat input as comma CSV and rewrite with semicolons.
		recs, err := readAllCSV(data, ',')
		if err != nil {
			return checks.FixResult{
				Data:      a.Data,
				Path:      "",
				DidChange: false,
				Note:      "",
			}, err
		}

		out, err := writeCSV(recs, ';')
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

	case tabOK:
		// Same story for tab-separated TSV.
		recs, err := readAllCSV(data, '\t')
		if err != nil {
			return checks.FixResult{
				Data:      a.Data,
				Path:      "",
				DidChange: false,
				Note:      "",
			}, err
		}

		out, err := writeCSV(recs, ';')
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
	}

	// 3. Not semicolon, not clearly comma/tab.
	//    Now we treat "mixed" as unsafe and bail.
	if hasMixedDelimiters(data) {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "detected inconsistent/mixed delimiters (e.g. both ',' and ';'); cannot safely auto-convert",
		}, checks.ErrNoFix
	}

	// 4. We couldn't determine a consistent alternative delimiter.
	return checks.FixResult{
		Data:      a.Data,
		Path:      "",
		DidChange: false,
		Note:      "",
	}, checks.ErrNoFix
}

// readAllCSV parses all records with the given delimiter.
func readAllCSV(data string, delim rune) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	return r.ReadAll()
}

// writeCSV serializes records with the given delimiter (';' for target).
func writeCSV(recs [][]string, delim rune) ([]byte, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	w.Comma = delim
	if err := w.WriteAll(recs); err != nil {
		return nil, err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	// encoding/csv doesn't append a final newline automatically per RFC; keep as produced.
	return []byte(b.String()), nil
}
