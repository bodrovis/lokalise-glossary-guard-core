package orphan_locale_descriptions

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// fixOrphanLocaleDescriptions inserts missing "<loc>" columns immediately BEFORE each
// "<loc>_description" column if "<loc>" is missing. Then it mirrors that structure
// into every data row by inserting "" in those new columns.
// It preserves all original data cells byte-for-byte (no TrimSpace on values).
func fixOrphanLocaleDescriptions(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	raw := string(a.Data)
	if raw == "" {
		return checks.NoFix(a, "no usable content to fix")
	}

	lines := strings.Split(raw, "\n")
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.NoFix(a, "no header line found")
	}

	headerLine := lines[headerIdx]
	if strings.TrimSpace(headerLine) == "" {
		return checks.NoFix(a, "empty header line")
	}

	// parse original header
	origHeaderCells := checks.SplitHeaderCells(headerLine)

	// mark existing (lowercase) columns
	existing := make(map[string]bool, len(origHeaderCells))
	for _, h := range origHeaderCells {
		existing[strings.ToLower(strings.TrimSpace(h))] = true
	}

	newHeader := make([]string, 0, len(origHeaderCells)+4)

	// locales мы уже добавили (чтобы не вставлять один и тот же base много раз)
	addedBase := make(map[string]bool)

	// track какие базы мы добавили вообще — для note
	var insertedLocales []string

	// pass 1: build newHeader with inserted base locales
	for _, col := range origHeaderCells {
		colTrim := strings.TrimSpace(col)
		colLC := strings.ToLower(colTrim)

		if strings.HasSuffix(colLC, "_description") {
			base := strings.TrimSuffix(colLC, "_description")
			base = strings.TrimSpace(base)

			if base != "" {
				// если в исходном хедере такого base не было и мы его ещё не добавляли в newHeader → вставляем
				if !existing[base] && !addedBase[base] {
					newHeader = append(newHeader, base)
					addedBase[base] = true
					existing[base] = true // чтобы следующие такие же *_description не дублировали вставку
					insertedLocales = append(insertedLocales, base)
				}
			}
		}

		// сам оригинальный столбец всегда идёт
		newHeader = append(newHeader, colTrim)
	}

	if len(insertedLocales) == 0 {
		return checks.NoFix(a, "no orphan *_description columns to fix")
	}

	// индекс хедера по имени, чтобы потом быстро находить значения в строках
	headerIndex := buildHeaderIndexMap(origHeaderCells)

	newLines := make([]string, 0, len(lines))

	// copy pre-header lines verbatim
	for i := 0; i < headerIdx; i++ {
		newLines = append(newLines, lines[i])
	}

	// write updated header
	newLines = append(newLines, strings.Join(newHeader, ";"))

	// rebuild data rows
	for row := headerIdx + 1; row < len(lines); row++ {
		rawRow := lines[row]

		// пустые строки сохраняем как есть (не лезем)
		if strings.TrimSpace(rawRow) == "" {
			newLines = append(newLines, rawRow)
			continue
		}

		values := splitRowCellsRaw(rawRow)

		newRowVals := make([]string, 0, len(newHeader))

		// addedThisRow следит, чтобы при первом orphan-е base мы вставили пустую ячейку,
		// а при следующем таком же orphan-е в этой же строке не вставляли повторно.
		addedThisRow := make(map[string]bool)

		for _, col := range origHeaderCells {
			colTrim := strings.TrimSpace(col)
			colLC := strings.ToLower(colTrim)

			if strings.HasSuffix(colLC, "_description") {
				base := strings.TrimSuffix(colLC, "_description")
				base = strings.TrimSpace(base)

				// если base не было в оригинальном хедере → мы добавили его в newHeader прямо перед этим *_description,
				// значит и тут надо вставить пустую клетку один раз
				if base != "" && !columnExistsInOriginalHeader(base, origHeaderCells) && !addedThisRow[base] {
					newRowVals = append(newRowVals, "")
					addedThisRow[base] = true
				}
			}

			// теперь само значение из оригинальной колонки
			if origIdx, ok := headerIndex[colTrim]; ok && origIdx < len(values) {
				newRowVals = append(newRowVals, values[origIdx])
			} else {
				newRowVals = append(newRowVals, "")
			}
		}

		newLines = append(newLines, strings.Join(newRowVals, ";"))
	}

	out := strings.Join(newLines, "\n")

	// build note like: "added missing locale columns before *_description: en, fr, zh-Hans"
	seenLoc := make(map[string]bool)
	locList := make([]string, 0, len(insertedLocales))
	for _, loc := range insertedLocales {
		if !seenLoc[loc] {
			seenLoc[loc] = true
			locList = append(locList, loc)
		}
	}
	note := "added missing locale columns before *_description: " + strings.Join(locList, ", ")

	return checks.FixResult{
		Data:      []byte(out),
		Path:      "",
		DidChange: true,
		Note:      note,
	}, nil
}

// splitRowCellsRaw splits row into cells WITHOUT trimming. We don't want to eat user spaces.
func splitRowCellsRaw(s string) []string {
	return strings.Split(s, ";")
}

// columnExistsInOriginalHeader checks case-insensitive presence of `base` in original header cells.
func columnExistsInOriginalHeader(base string, headerCells []string) bool {
	baseLC := strings.ToLower(strings.TrimSpace(base))
	for _, h := range headerCells {
		if strings.ToLower(strings.TrimSpace(h)) == baseLC {
			return true
		}
	}
	return false
}

// buildHeaderIndexMap returns map[trimmedHeaderName] = firstIndex
// so we don't do O(n²) lookups per row later.
func buildHeaderIndexMap(headerCells []string) map[string]int {
	m := make(map[string]int, len(headerCells))
	for i, h := range headerCells {
		key := strings.TrimSpace(h)
		if _, ok := m[key]; !ok {
			m[key] = i
		}
	}
	return m
}
