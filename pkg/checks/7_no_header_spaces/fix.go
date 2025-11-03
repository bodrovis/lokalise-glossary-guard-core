package no_spaces_in_header

import (
	"bytes"
	"context"
	"encoding/csv"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixNoSpacesInHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in := a.Data
	if len(in) == 0 {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "no usable content to trim header"}, checks.ErrNoFix
	}

	// preserve BOM + line ending + final NL
	bom := []byte{}
	if bytes.HasPrefix(in, []byte{0xEF, 0xBB, 0xBF}) {
		bom, in = []byte{0xEF, 0xBB, 0xBF}, in[3:]
	}
	lineSep := checks.DetectLineEnding(in) // "\r\n" | "\n" (как в предыдущих фикcах)
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	// выделяем первую строку целиком, остальное не трогаем
	firstLineEnd := bytes.IndexByte(in, '\n')
	var headerLine []byte
	var rest []byte
	switch firstLineEnd {
	case -1:
		headerLine = in
		rest = nil
	default:
		headerLine = in[:firstLineEnd] // без '\n'
		rest = in[firstLineEnd+1:]     // всё после первой строки
		if len(headerLine) > 0 && headerLine[len(headerLine)-1] == '\r' {
			headerLine = headerLine[:len(headerLine)-1] // срежем CR, если CRLF
		}
	}

	// распарсим только хедер csv.Reader-ом с разделителем ';'
	r := csv.NewReader(bytes.NewReader(headerLine))
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // не жёстко

	record, err := r.Read()
	if err != nil {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "cannot parse header; skip"}, checks.ErrNoFix
	}

	changed := false
	for i, v := range record {
		trim := bytes.TrimSpace([]byte(v))
		if !bytes.Equal([]byte(v), trim) {
			record[i] = string(trim)
			changed = true
		}
	}
	if !changed {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "header already trimmed"}, nil
	}

	// соберём новый хедер корректным CSV-сериализатором
	var hb bytes.Buffer
	w := csv.NewWriter(&hb)
	w.Comma = ';'
	if err := w.Write(record); err != nil {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to write trimmed header: " + err.Error()}, err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to flush header: " + err.Error()}, err
	}

	// csv.Writer всегда пишет '\n' в конце записи — приведём к исходному разделителю строк
	newHeader := hb.Bytes()
	if lineSep == "\r\n" {
		newHeader = bytes.ReplaceAll(newHeader, []byte("\n"), []byte("\r\n"))
	}

	if !keepFinal && len(rest) == 0 {
		if lineSep == "\r\n" && bytes.HasSuffix(newHeader, []byte("\r\n")) {
			newHeader = newHeader[:len(newHeader)-2]
		} else if lineSep == "\n" && bytes.HasSuffix(newHeader, []byte("\n")) {
			newHeader = newHeader[:len(newHeader)-1]
		}
	}

	// склеиваем: BOM + новый хедер + исходный хвост
	out := make([]byte, 0, len(bom)+len(newHeader)+len(rest)+2)
	out = append(out, bom...)
	out = append(out, newHeader...)
	out = append(out, rest...)

	// восстановим финальный перевод строки, если он был, а мы его потеряли
	if keepFinal && !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, []byte(lineSep)...)
	}

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "trimmed leading/trailing spaces in header cells",
	}, nil
}
