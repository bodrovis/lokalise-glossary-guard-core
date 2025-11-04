package allowed_columns_header

import (
	"bytes"
	"context"
	"encoding/csv"
	"slices"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixAllowedColumnsHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}
	in := a.Data
	if len(bytes.TrimSpace(in)) == 0 {
		return checks.NoFix(a, "no usable content to fix header")
	}

	// Preserve BOM
	var bom []byte
	if bytes.HasPrefix(in, []byte{0xEF, 0xBB, 0xBF}) {
		bom, in = in[:3], in[3:]
	}

	// Detect line ending + final newline presence (for the whole file)
	lineSep := checks.DetectLineEnding(in) // "\r\n" | "\n"
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	// ——— locate the first non-empty line as header; keep everything before it verbatim
	headerStart := 0
	found := false
	pos := 0
	for pos <= len(in) {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		nlRel := bytes.IndexByte(in[pos:], '\n')
		var line []byte
		nextPos := len(in)
		if nlRel >= 0 {
			line = in[pos : pos+nlRel] // without '\n'
			nextPos = pos + nlRel + 1  // skip '\n'
		} else {
			line = in[pos:]
		}
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1] // strip CR of CRLF
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
	after := in[headerStart:] // starts at header (как и было)

	// Parse tail (from header) as CSV with semicolons
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

	// header record is the first non-empty row among `records`
	hIdx := -1
	for i, rec := range records {
		if checks.AnyNonEmpty(rec) {
			hIdx = i
			break
		}
	}
	if hIdx < 0 {
		return checks.NoFix(a, "no header record found")
	}

	header := records[hIdx]
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

	// Declared languages (dedup, lowered)
	declaredOrder := make([]string, 0, len(a.Langs))
	seenDecl := map[string]struct{}{}
	for _, l := range a.Langs {
		ll := norm(l)
		if ll == "" {
			continue
		}
		if _, ok := seenDecl[ll]; !ok {
			seenDecl[ll] = struct{}{}
			declaredOrder = append(declaredOrder, ll)
		}
	}
	hasDeclared := len(declaredOrder) > 0

	// Build keep list from original header: keep only allowed columns
	// Allowed: coreAllowed[...] OR language columns that match declared languages (base or base_description)
	type colInfo struct {
		label string // original label to keep in header
		idx   int    // original index in row (for data remap), -1 for newly added
	}

	var keep []colInfo
	seenLangPresence := map[string]struct {
		base bool
		desc bool
	}{}

	for j, name := range header {
		n := norm(name)
		// core allowed?
		if _, ok := checks.KnownHeaders[n]; ok {
			keep = append(keep, colInfo{label: name, idx: j})
			continue
		}
		// language-like?
		base, isLang := parseLangColumn(name) // expects original helper: returns base (without _description) and true/false
		if !isLang {
			// drop unknown column
			continue
		}
		baseL := norm(base)
		// keep only if declared (when we have declared set)
		if hasDeclared && !contains(declaredOrder, baseL) {
			continue
		}
		// normalize exact label form for _description part, but do NOT lowercase here
		if strings.EqualFold(n, baseL) {
			keep = append(keep, colInfo{label: name, idx: j})
			st := seenLangPresence[baseL]
			st.base = true
			seenLangPresence[baseL] = st
			continue
		}
		if strings.EqualFold(n, baseL+"_description") {
			// normalize suffix casing to "_description" if автор писал криво
			lbl := base + "_description"
			if !strings.HasSuffix(name, "_description") {
				name = lbl
			}
			keep = append(keep, colInfo{label: name, idx: j})
			st := seenLangPresence[baseL]
			st.desc = true
			seenLangPresence[baseL] = st
			continue
		}
		// anything else — drop
	}

	// Ensure declared languages have both base & desc columns
	if hasDeclared {
		for _, lang := range declaredOrder {
			st := seenLangPresence[lang]
			if !st.base {
				keep = append(keep, colInfo{label: lang, idx: -1})
				st.base = true
			}
			if !st.desc {
				keep = append(keep, colInfo{label: lang + "_description", idx: -1})
				st.desc = true
			}
			seenLangPresence[lang] = st
		}
	}

	// If nothing changes (same labels in same order), bail out
	same := len(keep) == len(header)
	if same {
		for i := range keep {
			if norm(keep[i].label) != norm(header[i]) {
				same = false
				break
			}
		}
	}
	if same {
		return checks.FixResult{
			Data:      a.Data,
			Path:      a.Path,
			DidChange: false,
			Note:      "header already normalized",
		}, nil
	}

	// Rebuild records: header & all rows remapped
	newHeader := make([]string, len(keep))
	for i := range keep {
		newHeader[i] = keep[i].label
	}

	outRecs := make([][]string, len(records))
	for i := 0; i < len(records); i++ {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		row := records[i]
		// keep blank records as true blank (no fields) to not create fake separators
		if !checks.AnyNonEmpty(row) && i < hIdx {
			outRecs[i] = nil
			continue
		}
		newRow := make([]string, len(keep))
		for j, c := range keep {
			if c.idx >= 0 && c.idx < len(row) {
				newRow[j] = row[c.idx]
			} else {
				newRow[j] = ""
			}
		}
		outRecs[i] = newRow
	}
	// ensure header row has labels (even if it was blank-ish)
	outRecs[hIdx] = slices.Clone(newHeader)

	// Serialize back
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	for i := 0; i < len(outRecs); i++ {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		if outRecs[i] == nil {
			// write a blank line (no fields)
			if _, err := buf.WriteString(lineSep); err != nil {
				return checks.FixResult{Data: a.Data, Path: a.Path, DidChange: false, Note: "failed to write blank line"}, err
			}
			continue
		}
		if err := w.Write(outRecs[i]); err != nil {
			return checks.FixResult{Data: a.Data, Path: a.Path, DidChange: false, Note: "failed to serialize CSV: " + err.Error()}, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return checks.FixResult{Data: a.Data, Path: a.Path, DidChange: false, Note: "failed to flush CSV: " + err.Error()}, err
	}

	outTail := buf.Bytes()
	// csv.Writer использует '\n' — приведём к исходному lineSep
	if lineSep == "\r\n" {
		outTail = bytes.ReplaceAll(outTail, []byte("\n"), []byte("\r\n"))
	}

	// Сохраним отсутствие финального NL, если его не было и хвоста мы сняли
	if !keepFinal && bytes.HasSuffix(outTail, []byte(lineSep)) {
		outTail = outTail[:len(outTail)-len(lineSep)]
	}

	// Склеить: BOM + before (как было) + outTail
	out := make([]byte, 0, len(bom)+len(before)+len(outTail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, outTail...)

	return checks.FixResult{
		Data:      out,
		Path:      a.Path,
		DidChange: true,
		Note:      "removed unknown columns and ensured declared languages are present",
	}, nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
