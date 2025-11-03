package duplicate_header_cells

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// fixDuplicateHeaderCells removes duplicate columns (2nd+ occurrences) from header and all data rows.
// Keeps first occurrence of each normalized header cell. Also drops duplicate empty headers "".
func fixDuplicateHeaderCells(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
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
	lineSep := checks.DetectLineEnding(in) // "\r\n" | "\n"
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	// Find first non-empty line (header start); keep everything before it verbatim
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
			line = in[pos : pos+nlRel] // exclude '\n'
			nextPos = pos + nlRel + 1  // skip '\n'
		} else {
			line = in[pos:]
		}
		// strip trailing '\r' for CRLF
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
	after := in[headerStart:] // starts at header

	// Parse CSV (from header) with semicolons
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

	// header is the first record here (we started at first non-empty line)
	header := records[0]
	if len(header) == 0 {
		return checks.NoFix(a, "empty header record")
	}

	// Decide which column indices to keep (first occurrence per normalized name)
	seen := make(map[string]bool, len(header))
	keepIdx := make([]int, 0, len(header))
	removedNames := make([]string, 0)

	for i, col := range header {
		name := strings.TrimSpace(col)
		lc := strings.ToLower(name)
		if seen[lc] {
			removedNames = append(removedNames, name)
			continue
		}
		seen[lc] = true
		keepIdx = append(keepIdx, i)
	}

	if len(removedNames) == 0 {
		return checks.NoFix(a, "no duplicate header columns to remove")
	}

	// Rebuild all rows according to keepIdx (moves entire columns)
	outRecs := make([][]string, len(records))
	for i := 0; i < len(records); i++ {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		row := records[i]
		newRow := make([]string, len(keepIdx))
		for j, idx := range keepIdx {
			if idx >= 0 && idx < len(row) {
				newRow[j] = row[idx]
			} else {
				newRow[j] = ""
			}
		}
		outRecs[i] = newRow
	}

	// Serialize back
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	for i := 0; i < len(outRecs); i++ {
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

	outTail := buf.Bytes() // uses '\n'
	// normalize line endings
	if lineSep == "\r\n" {
		outTail = bytes.ReplaceAll(outTail, []byte("\n"), []byte("\r\n"))
	}
	// preserve absence of final NL if originally none
	if !keepFinal && bytes.HasSuffix(outTail, []byte(lineSep)) {
		outTail = outTail[:len(outTail)-len(lineSep)]
	}

	// stitch: BOM + before + converted tail
	out := make([]byte, 0, len(bom)+len(before)+len(outTail))
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, outTail...)

	note := "removed duplicate header columns: " + strings.Join(removedNames, ", ")
	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      note,
	}, nil
}
