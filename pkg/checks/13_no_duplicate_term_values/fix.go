package duplicate_term_values

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixDuplicateTermValues(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in := a.Data
	if len(bytes.TrimSpace(in)) == 0 {
		return checks.NoFix(a, "no usable content to fix")
	}

	// BOM
	var bom []byte
	if bytes.HasPrefix(in, []byte{0xEF, 0xBB, 0xBF}) {
		bom, in = in[:3], in[3:]
	}

	// line endings / final NL
	lineSep := checks.DetectLineEnding(in) // "\r\n" | "\n"
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

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
			line = line[:len(line)-1]
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
		return checks.NoFix(a, "no header with 'term' column found")
	}

	before := in[:headerStart]
	after := in[headerStart:]

	headerLineNo := 1 + bytes.Count(before, []byte("\n"))

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

	hIdx := -1
	for i, rec := range records {
		if checks.AnyNonEmpty(rec) {
			hIdx = i
			break
		}
	}
	if hIdx < 0 {
		return checks.NoFix(a, "no header with 'term' column found")
	}

	termCol := -1
	for j, h := range records[hIdx] {
		if strings.EqualFold(strings.TrimSpace(h), "term") {
			termCol = j
			break
		}
	}
	if termCol < 0 {
		return checks.NoFix(a, "no 'term' column found")
	}

	seen := make(map[string]bool)
	type removedInfo struct{ rows []int }
	removed := map[string]*removedInfo{}

	outRecs := make([][]string, 0, len(records))
	outRecs = append(outRecs, records[hIdx])

	for i := hIdx + 1; i < len(records); i++ {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		rec := records[i]

		val := ""
		if termCol < len(rec) {
			val = strings.TrimSpace(rec[termCol])
		}
		if val == "" {
			outRecs = append(outRecs, rec)
			continue
		}
		if !seen[val] {
			seen[val] = true
			outRecs = append(outRecs, rec)
			continue
		}

		info := removed[val]
		if info == nil {
			info = &removedInfo{}
			removed[val] = info
		}
		info.rows = append(info.rows, headerLineNo+i)
	}

	if len(removed) == 0 {
		return checks.NoFix(a, "no duplicate term rows to remove")
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'

	for _, rec := range outRecs {
		if err := w.Write(rec); err != nil {
			return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to serialize CSV: " + err.Error()}, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to flush CSV: " + err.Error()}, err
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

	var nb strings.Builder
	nb.WriteString("removed duplicate term rows for: ")
	first := true
	for term, info := range removed {
		if !first {
			nb.WriteString("; ")
		}
		first = false
		nb.WriteString(`"`)
		nb.WriteString(term)
		nb.WriteString(`" (rows `)
		nb.WriteString(joinInts(info.rows, ", "))
		nb.WriteString(`)`)
	}

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      nb.String(),
	}, nil
}

// helpers

func joinInts(is []int, sep string) string {
	if len(is) == 0 {
		return ""
	}
	var b strings.Builder
	for i, n := range is {
		if i > 0 {
			b.WriteString(sep)
			b.WriteString(" ")
		}
		b.WriteString(strconv.Itoa(n))
	}
	return b.String()
}
