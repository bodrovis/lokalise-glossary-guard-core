package term_description_header

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixTermDescriptionHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	raw := string(a.Data)
	if raw == "" {
		return noFix(a, "no usable content to fix")
	}

	lines := strings.Split(raw, "\n")
	headerIdx := firstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return noFix(a, "no header line found")
	}

	// исходный header и его колонки
	origHeaderLine := lines[headerIdx]
	origCols := splitCells(origHeaderLine)

	// найдём индексы существующих term/description
	posTerm, posDesc := findHeaderPositions(origCols)

	// если уже норм: term;description впереди
	if len(origCols) >= 2 && origCols[0] == "term" && origCols[1] == "description" {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "header already starts with term;description",
		}, nil
	}

	// сконструировать новый порядок колонок
	// первые две обязательные
	newCols := []string{"term", "description"}

	// потом добавляем все остальные колонки в исходном порядке, кроме term/description
	for _, col := range origCols {
		lc := strings.ToLower(strings.TrimSpace(col))
		if lc == "term" || lc == "description" {
			continue
		}
		newCols = append(newCols, col)
	}

	// заменяем хедер
	lines[headerIdx] = strings.Join(newCols, ";")

	// теперь надо переставить значения в КАЖДОЙ строке данных по этому новому порядку
	for i := headerIdx + 1; i < len(lines); i++ {
		rowRaw := lines[i]
		// не трогаем полностью пустые/пробельные строки
		if strings.TrimSpace(rowRaw) == "" {
			continue
		}

		values := splitCells(rowRaw)

		// соберём новую строку значений по newCols
		newValues := make([]string, len(newCols))

		// 1) term -> столбец 0
		if posTerm >= 0 && posTerm < len(values) {
			newValues[0] = values[posTerm]
		} else {
			newValues[0] = ""
		}

		// 2) description -> столбец 1
		if posDesc >= 0 && posDesc < len(values) {
			newValues[1] = values[posDesc]
		} else {
			newValues[1] = ""
		}

		// 3) остальные колонки
		writeIdx := 2
		for j, colName := range origCols {
			lc := strings.ToLower(strings.TrimSpace(colName))
			if lc == "term" || lc == "description" {
				continue
			}

			if j < len(values) {
				newValues[writeIdx] = values[j]
			} else {
				newValues[writeIdx] = ""
			}
			writeIdx++
		}

		lines[i] = strings.Join(newValues, ";")
	}

	out := strings.Join(lines, "\n")

	return checks.FixResult{
		Data:      []byte(out),
		Path:      "",
		DidChange: true,
		Note:      "normalized header and rows to start with term;description",
	}, nil
}

func splitCells(s string) []string {
	parts := strings.Split(s, ";")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func findHeaderPositions(cols []string) (int, int) {
	posTerm, posDesc := -1, -1
	for i, c := range cols {
		cc := strings.ToLower(strings.TrimSpace(c))
		switch cc {
		case "term":
			if posTerm < 0 {
				posTerm = i
			}
		case "description":
			if posDesc < 0 {
				posDesc = i
			}
		}
	}
	return posTerm, posDesc
}

func noFix(a checks.Artifact, note string) (checks.FixResult, error) {
	return checks.FixResult{
		Data:      a.Data,
		Path:      "",
		DidChange: false,
		Note:      note,
	}, checks.ErrNoFix
}
