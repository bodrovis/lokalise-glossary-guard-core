package allowed_columns_header

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

func fixAllowedColumnsHeader(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	raw := string(a.Data)
	if raw == "" {
		return checks.NoFix(a, "no usable content to fix header")
	}

	lines := strings.Split(raw, "\n")
	headerIdx := checks.FirstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.NoFix(a, "no header line found")
	}

	origHeader := lines[headerIdx]
	origCols := checks.SplitHeaderCells(origHeader)

	declaredSet := make(map[string]struct{})
	declaredOrder := make([]string, 0, len(a.Langs))
	for _, l := range a.Langs {
		ll := strings.ToLower(strings.TrimSpace(l))
		if ll == "" {
			continue
		}
		if _, seen := declaredSet[ll]; !seen {
			declaredSet[ll] = struct{}{}
			declaredOrder = append(declaredOrder, ll)
		}
	}
	hasDeclared := len(declaredOrder) > 0

	type langPresence struct {
		hasBase bool
		hasDesc bool
	}
	seenLangs := map[string]*langPresence{}

	newCols := make([]string, 0, len(origCols))
	keepIdx := make([]int, 0, len(origCols)) // maps newCols[j] -> index in origCols or -1 if new
	changed := false

	for idx, col := range origCols {
		colTrim := strings.TrimSpace(col)
		if colTrim == "" {
			if col != "" {
				changed = true
			}
			continue
		}

		colLower := strings.ToLower(colTrim)

		if _, ok := coreAllowed[colLower]; ok {
			newCols = append(newCols, colLower)
			keepIdx = append(keepIdx, idx)
			if colLower != colTrim {
				changed = true
			}
			continue
		}

		langBase, isLangLike := parseLangColumn(colTrim)
		if isLangLike {
			baseLower := strings.ToLower(langBase)
			p := seenLangs[baseLower]
			if p == nil {
				p = &langPresence{}
				seenLangs[baseLower] = p
			}

			if strings.EqualFold(colTrim, langBase) {
				p.hasBase = true
				newCols = append(newCols, colTrim)
				keepIdx = append(keepIdx, idx)
			} else if strings.HasSuffix(colTrim, "_description") {
				p.hasDesc = true

				basePart := strings.TrimSuffix(colTrim, "_description")
				normalized := basePart + "_description"
				if normalized != colTrim {
					changed = true
				}
				newCols = append(newCols, normalized)
				keepIdx = append(keepIdx, idx)
			} else {
				newCols = append(newCols, colTrim)
				keepIdx = append(keepIdx, idx)
			}

			if colTrim != col {
				changed = true
			}
			continue
		}

		changed = true
	}

	if hasDeclared {
		for _, lang := range declaredOrder {
			p := seenLangs[lang]

			if p == nil {
				newCols = append(newCols, lang)
				keepIdx = append(keepIdx, -1)

				newCols = append(newCols, lang+"_description")
				keepIdx = append(keepIdx, -1)

				changed = true
				continue
			}

			if !p.hasBase {
				newCols = append(newCols, lang)
				keepIdx = append(keepIdx, -1)
				changed = true
			}
			if !p.hasDesc {
				newCols = append(newCols, lang+"_description")
				keepIdx = append(keepIdx, -1)
				changed = true
			}
		}
	}

	if !changed {
		return checks.FixResult{
			Data:      a.Data,
			Path:      a.Path,
			DidChange: false,
			Note:      "header already normalized",
		}, nil
	}

	outLines := make([]string, 0, len(lines))

	for lineIdx, line := range lines {
		if lineIdx == headerIdx {
			outLines = append(outLines, strings.Join(newCols, ";"))
			continue
		}

		if line == "" {
			outLines = append(outLines, line)
			continue
		}

		rowCells := strings.Split(line, ";")

		newRow := make([]string, len(newCols))
		for j, origPos := range keepIdx {
			if origPos >= 0 {
				if origPos < len(rowCells) {
					newRow[j] = rowCells[origPos]
				} else {
					newRow[j] = ""
				}
			} else {
				newRow[j] = ""
			}
		}

		outLines = append(outLines, strings.Join(newRow, ";"))
	}

	outData := strings.Join(outLines, "\n")

	return checks.FixResult{
		Data:      []byte(outData),
		Path:      a.Path,
		DidChange: true,
		Note:      "removed unknown columns and ensured declared languages are present",
	}, nil
}
