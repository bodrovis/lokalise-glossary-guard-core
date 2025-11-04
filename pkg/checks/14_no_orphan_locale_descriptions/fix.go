package orphan_locale_descriptions

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixOrphanLocaleDescriptions(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
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

	// Line ending + final NL
	lineSep := checks.DetectLineEnding(in) // "\r\n" | "\n"
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	// Find first non-empty line (header)
	headerStart := 0
	found := false
	for pos := 0; pos <= len(in); {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		nlRel := bytes.IndexByte(in[pos:], '\n')
		var line []byte
		nextPos := len(in)
		if nlRel >= 0 {
			line = in[pos : pos+nlRel]
			nextPos = pos + nlRel + 1
		} else {
			line = in[pos:]
		}
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		if len(bytes.TrimSpace(line)) != 0 {
			headerStart = pos
			found = true
			break
		}
		if nlRel < 0 {
			break
		}
		pos = nextPos
	}
	if !found {
		return checks.NoFix(a, "no header line found")
	}

	before := in[:headerStart]
	after := in[headerStart:]

	r := csv.NewReader(bytes.NewReader(after))
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

	header := records[0]
	if !checks.AnyNonEmpty(header) {
		return checks.NoFix(a, "empty header line")
	}

	type colMap struct {
		label  string
		srcIdx int
	}
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

	origSet := make(map[string]bool, len(header))
	for _, h := range header {
		origSet[norm(h)] = true
	}

	var mapping []colMap
	addedBase := make(map[string]bool)
	insertedLocales := make([]string, 0, 8)

	for j, col := range header {
		colTrim := strings.TrimSpace(col)
		lc := norm(colTrim)

		if strings.HasSuffix(lc, "_description") {
			base := strings.TrimSuffix(lc, "_description")
			if base != "" && !origSet[base] && !addedBase[base] {
				mapping = append(mapping, colMap{label: base, srcIdx: -1})
				addedBase[base] = true
				insertedLocales = append(insertedLocales, base)
			}
		}

		mapping = append(mapping, colMap{label: colTrim, srcIdx: j})
	}

	if len(insertedLocales) == 0 {
		return checks.NoFix(a, "no orphan *_description columns to fix")
	}

	outRecs := make([][]string, len(records))
	for i := range len(records) {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		row := records[i]
		newRow := make([]string, len(mapping))
		if i == 0 {
			// хедер — именно названия колонок из mapping
			for k, m := range mapping {
				newRow[k] = m.label
			}
		} else {
			// данные — берем из srcIdx или пусто
			for k, m := range mapping {
				if m.srcIdx >= 0 && m.srcIdx < len(row) {
					newRow[k] = row[m.srcIdx]
				} else {
					newRow[k] = ""
				}
			}
		}
		outRecs[i] = newRow
	}

	// Serialize
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	for i := range len(outRecs) {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		if err := w.Write(outRecs[i]); err != nil {
			return checks.FixResult{
				Data:      a.Data,
				Path:      "",
				DidChange: false,
				Note:      "failed to serialize CSV: " + err.Error(),
			}, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "failed to flush CSV: " + err.Error(),
		}, err
	}

	outTail := buf.Bytes()

	if lineSep == "\r\n" {
		outTail = bytes.ReplaceAll(outTail, []byte("\n"), []byte("\r\n"))
	}

	if !keepFinal && bytes.HasSuffix(outTail, []byte(lineSep)) {
		outTail = outTail[:len(outTail)-len(lineSep)]
	}

	out := make([]byte, 0, len(bom)+len(before)+len(outTail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, outTail...)

	seen := map[string]struct{}{}
	locList := make([]string, 0, len(insertedLocales))
	for _, loc := range insertedLocales {
		if _, ok := seen[loc]; !ok {
			seen[loc] = struct{}{}
			locList = append(locList, loc)
		}
	}
	note := "added missing locale columns before *_description: " + strings.Join(locList, ", ")

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      note,
	}, nil
}
