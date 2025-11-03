package invalid_flags

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixNoInvalidFlags(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
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
	// Line endings / final NL
	lineSep := checks.DetectLineEnding(in) // "\r\n" | "\n"
	keepFinal := bytes.HasSuffix(in, []byte("\n"))

	// Найти старт первой непустой строки (хедер); всё до неё — как есть
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
			line = in[pos : pos+nlRel]
			nextPos = pos + nlRel + 1
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
	after := in[headerStart:] // начинается с хедера

	// Парсим CSV с ';'
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

	// Хедер — первая запись тут
	header := records[0]
	if !checks.AnyNonEmpty(header) {
		return checks.NoFix(a, "empty header line")
	}

	// Индексы наблюдаемых колонок
	colPos := make(map[string]int, len(watchedCols))
	for _, w := range watchedCols {
		colPos[w] = -1
	}
	for i, h := range header {
		lc := strings.ToLower(strings.TrimSpace(h))
		if _, ok := colPos[lc]; ok && colPos[lc] == -1 {
			colPos[lc] = i
		}
	}
	allMissing := true
	for _, w := range watchedCols {
		if colPos[w] >= 0 {
			allMissing = false
			break
		}
	}
	if allMissing {
		return checks.NoFix(a, "no flag columns to normalize")
	}

	changed := false
	outRecs := make([][]string, len(records))

	// Копируем хедер как есть
	outRecs[0] = records[0]

	// Остальные строки — нормализуем только флаговые колонки
	for i := 1; i < len(records); i++ {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		row := records[i]
		// пустые строки сохраняем как пустую запись (запишем blank line)
		if !checks.AnyNonEmpty(row) {
			outRecs[i] = nil
			continue
		}
		newRow := make([]string, len(row))
		copy(newRow, row)

		for _, w := range watchedCols {
			idx := colPos[w]
			if idx < 0 || idx >= len(newRow) {
				continue
			}
			orig := newRow[idx]
			norm := normalizeFlagValue(orig)
			if norm != orig {
				newRow[idx] = norm
				changed = true
			}
		}
		outRecs[i] = newRow
	}

	if !changed {
		return checks.NoFix(a, "no flag values to normalize")
	}

	// Сериализуем назад
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	for _, rec := range outRecs {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		if rec == nil {
			// чистая пустая строка
			if _, err := buf.WriteString(lineSep); err != nil {
				return checks.FixResult{Data: a.Data, Path: "", DidChange: false, Note: "failed to write blank line"}, err
			}
			continue
		}
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

	return checks.FixResult{
		Data:      out,
		Path:      "",
		DidChange: true,
		Note:      "normalized flag columns to yes/no",
	}, nil
}

// как у тебя: trim для принятия решения, но остальные ячейки не трогаем
func normalizeFlagValue(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return v
	}
	switch strings.ToLower(trimmed) {
	case "yes", "y", "true", "1":
		return "yes"
	case "no", "n", "false", "0":
		return "no"
	default:
		return v
	}
}
