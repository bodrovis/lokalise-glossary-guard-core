package term_description_header

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixTermDescriptionHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}
	in := a.Data
	if len(bytes.TrimSpace(in)) == 0 {
		return checks.NoFix(a, "no usable content to fix")
	}

	// Preserve BOM
	var bom []byte
	if bytes.HasPrefix(in, []byte{0xEF, 0xBB, 0xBF}) {
		bom, in = in[:3], in[3:]
	}

	// Detect original line ending + final newline presence
	lineSep := checks.DetectLineEnding(in) // "\r\n" or "\n" (same helper as в других фикcах)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	// Parse entire CSV with semicolons
	r := csv.NewReader(bytes.NewReader(in))
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	records, err := r.ReadAll()
	if err != nil || len(records) == 0 {
		if ctx.Err() != nil {
			return checks.FixResult{}, ctx.Err()
		}
		return checks.NoFix(a, "cannot parse CSV with semicolon delimiter")
	}

	// Find header record index: первая запись с хоть каким-то непустым значением
	hIdx := -1
	for i, rec := range records {
		nonEmpty := false
		for _, v := range rec {
			if strings.TrimSpace(v) != "" {
				nonEmpty = true
				break
			}
		}
		if nonEmpty {
			hIdx = i
			break
		}
	}
	if hIdx < 0 {
		return checks.NoFix(a, "no header record found")
	}

	header := records[hIdx]
	// map from normalized name -> first index
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

	pos := make(map[string]int, len(header))
	for i, name := range header {
		n := norm(name)
		if _, ok := pos[n]; !ok {
			pos[n] = i
		}
	}
	posTerm, okTerm := pos["term"]
	posDesc, okDesc := pos["description"]

	// Build new header order:
	// start with term, description (add if absent), then all other original columns in original order (skipping term/description)
	newHeader := make([]string, 0, len(header)+2)
	newHeader = append(newHeader, "term", "description")
	for _, name := range header {
		n := norm(name)
		if n == "term" || n == "description" {
			continue
		}
		newHeader = append(newHeader, name)
	}

	// If header already starts with term;description exactly -> no change
	if len(header) >= 2 && norm(header[0]) == "term" && norm(header[1]) == "description" {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "header already starts with term;description",
		}, nil
	}

	// Rebuild all rows according to newHeader
	outRecs := make([][]string, len(records))
	copy(outRecs, records) // we'll overwrite per-row

	// Mapping for the "rest" columns (excluding term/description) by original index
	type colInfo struct {
		name string
		idx  int
	}
	restCols := make([]colInfo, 0, len(header))
	for j, name := range header {
		n := norm(name)
		if n == "term" || n == "description" {
			continue
		}
		restCols = append(restCols, colInfo{name: name, idx: j})
	}

	for i := 0; i < len(records); i++ {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		row := records[i]
		// New row length equals new header length
		newRow := make([]string, len(newHeader))

		// term -> col 0
		if okTerm && posTerm < len(row) {
			newRow[0] = row[posTerm]
		} else {
			newRow[0] = ""
		}
		// description -> col 1
		if okDesc && posDesc < len(row) {
			newRow[1] = row[posDesc]
		} else {
			newRow[1] = ""
		}
		// remaining columns in original order (skip term/description)
		w := 2
		for _, c := range restCols {
			if c.idx < len(row) {
				newRow[w] = row[c.idx]
			} else {
				newRow[w] = ""
			}
			w++
		}
		outRecs[i] = newRow
	}
	// Replace header record with newHeader labels (ensure exact labels)
	outRecs[hIdx][0] = "term"
	outRecs[hIdx][1] = "description"
	for k := 2; k < len(newHeader); k++ {
		outRecs[hIdx][k] = newHeader[k]
	}

	// Note: if term/description were absent, we just added empty columns for data rows — это ожидаемо.

	// Serialize back
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	for i := 0; i < len(outRecs); i++ {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		if err := w.Write(outRecs[i]); err != nil {
			return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to serialize CSV: " + err.Error()}, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to flush CSV: " + err.Error()}, err
	}

	out := buf.Bytes() // ends with '\n' per record
	// Restore original line endings
	if lineSep == "\r\n" {
		out = bytes.ReplaceAll(out, []byte("\n"), []byte("\r\n"))
	}
	// Restore absence of final NL if originally none
	if !keepFinal && len(out) >= len(lineSep) && bytes.HasSuffix(out, []byte(lineSep)) {
		out = out[:len(out)-len(lineSep)]
	}
	// Prepend BOM back
	if len(bom) > 0 {
		tmp := make([]byte, 0, len(bom)+len(out))
		tmp = append(tmp, bom...)
		tmp = append(tmp, out...)
		out = tmp
	}

	note := "reordered columns to start with term;description"
	if !okTerm || !okDesc {
		note = "inserted missing term/description columns at start"
	}

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      note,
	}, nil
}
