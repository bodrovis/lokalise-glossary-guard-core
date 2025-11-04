package lowercase_header

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixLowercaseHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}
	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no usable content to normalize header",
		}, checks.ErrNoFix
	}

	in := a.Data

	// preserve BOM
	var bom []byte
	if bytes.HasPrefix(in, []byte{0xEF, 0xBB, 0xBF}) {
		bom, in = in[:3], in[3:]
	}

	// detect original line ending + final newline presence
	lineSep := checks.DetectLineEnding(in) // "\r\n" | "\n"
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	// find first non-empty line (preserve leading whitespace/blank lines)
	var headerStart, headerEnd int
	found := false
	pos := 0
	for pos <= len(in) {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		// locate next '\n'
		nlRel := bytes.IndexByte(in[pos:], '\n')
		var line []byte
		var nextPos int
		switch nlRel {
		case -1:
			line = in[pos:]
			nextPos = len(in)
		default:
			line = in[pos : pos+nlRel] // exclude '\n'
			nextPos = pos + nlRel + 1  // skip '\n'
		}
		// strip trailing '\r' for CRLF
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(bytes.TrimSpace(line)) != 0 {
			headerStart = pos
			headerEnd = nextPos // include original newline if there был
			found = true
			break
		}
		if nextPos >= len(in) {
			break
		}
		pos = nextPos
	}
	if !found {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no header line found",
		}, checks.ErrNoFix
	}

	// slice parts
	headerLine := in[headerStart:headerEnd]
	// drop newline + optional CR from headerLine for parsing
	if m := len(headerLine); m > 0 {
		// remove trailing '\n'
		if headerLine[m-1] == '\n' {
			headerLine = headerLine[:m-1]
			// remove trailing '\r' (CRLF)
			if k := len(headerLine); k > 0 && headerLine[k-1] == '\r' {
				headerLine = headerLine[:k-1]
			}
		}
	}

	before := in[:headerStart]
	rest := in[headerEnd:]

	// csv-parse header
	r := csv.NewReader(bytes.NewReader(headerLine))
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	record, err := r.Read()
	if err != nil || len(record) == 0 {
		if ctx.Err() != nil {
			return checks.FixResult{}, ctx.Err()
		}
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "cannot parse header with semicolon delimiter",
		}, checks.ErrNoFix
	}

	// normalize only known columns to lowercase
	changed := false
	for i, c := range record {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		t := strings.TrimSpace(c)
		if t == "" {
			continue
		}
		lc := strings.ToLower(t)
		if _, ok := checks.KnownHeaders[lc]; ok && c != lc {
			record[i] = lc
			changed = true
		}
	}
	if !changed {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "header service columns already lowercase",
		}, nil
	}

	// serialize header
	var hb bytes.Buffer
	w := csv.NewWriter(&hb)
	w.Comma = ';'
	if err := w.Write(record); err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "failed to serialize normalized header",
		}, err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "failed to flush normalized header",
		}, err
	}
	newHeader := hb.Bytes() // ends with '\n'

	// normalize line endings to original
	if lineSep == "\r\n" {
		newHeader = bytes.ReplaceAll(newHeader, []byte("\n"), []byte("\r\n"))
	}

	// if file has only header line and originally had no final NL — remove the one csv.Writer added
	if !keepFinal && len(rest) == 0 {
		if lineSep == "\r\n" && bytes.HasSuffix(newHeader, []byte("\r\n")) {
			newHeader = newHeader[:len(newHeader)-2]
		} else if lineSep == "\n" && bytes.HasSuffix(newHeader, []byte("\n")) {
			newHeader = newHeader[:len(newHeader)-1]
		}
	}

	// stitch: BOM + before + newHeader + rest
	out := make([]byte, 0, len(bom)+len(before)+len(newHeader)+len(rest)+2)
	out = append(out, bom...)
	out = append(out, before...)
	out = append(out, newHeader...)
	out = append(out, rest...)

	// restore final newline if it originally existed
	if keepFinal && !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, []byte(lineSep)...)
	}

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "normalized service columns in header to lowercase",
	}, nil
}
