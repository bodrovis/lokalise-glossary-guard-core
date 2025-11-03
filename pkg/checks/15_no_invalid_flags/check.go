package invalid_flags

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-invalid-flags"

// columns we treat as boolean flags
var watchedCols = []string{
	"casesensitive",
	"translatable",
	"forbidden",
}

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoInvalidFlags,
		checks.WithFailFast(),   // hard fail if bad values appear
		checks.WithPriority(15), // runs after all structural cleanup
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoInvalidFlags(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateNoInvalidFlags,
		Fix:              fixNoInvalidFlags,
		PassMsg:          "all flag columns contain only yes/no",
		FixedMsg:         "normalized flag columns to yes/no",
		AppliedMsg:       "auto-fix applied: normalized flag columns to yes/no",
		StatusAfterFixed: checks.Pass,
		// FailAs defaults to FAIL (hard fail)
		StillBadMsg: "invalid flag values remain after fix",
	})
}

// validateNoInvalidFlags checks that all watchedCols (if present in header)
// only contain "yes" or "no" (case-sensitive) and not empty.

func validateNoInvalidFlags(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: true, Msg: "no content to validate for flags"}
	}

	// CSV reader с разделителем ';'
	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	// читаем первую НЕПУСТУЮ запись как хедер
	var header []string
	recIdx := 0 // 1-based индекс записи
	for {
		rec, err := r.Read()
		if err != nil {
			if ctx.Err() != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
			}
			return checks.ValidationResult{OK: true, Msg: "no header line found (nothing to validate for flags)"}
		}
		recIdx++
		nonEmpty := false
		for _, c := range rec {
			if strings.TrimSpace(c) != "" {
				nonEmpty = true
				break
			}
		}
		if nonEmpty {
			header = rec
			break
		}
	}

	// map watchedCol (lc) -> index в хедере, -1 если нет
	colPos := make(map[string]int, len(watchedCols))
	for _, w := range watchedCols {
		colPos[strings.ToLower(w)] = -1
	}
	for i, h := range header {
		lc := strings.ToLower(strings.TrimSpace(h))
		if _, ok := colPos[lc]; ok {
			colPos[lc] = i
		}
	}

	type bad struct {
		colName string
		val     string
		rowNum  int // 1-based CSV-строка
	}
	var invalids []bad

	rowNum := recIdx
	const checkEvery = 1 << 12
	for {
		if (rowNum % checkEvery) == 0 {
			if err := ctx.Err(); err != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
			}
		}
		rec, err := r.Read()
		if err != nil {
			break // EOF/ошибка парса — другие чеки отрапортят
		}
		rowNum++

		// пропускаем полностью пустые строки
		allEmpty := true
		for _, c := range rec {
			if strings.TrimSpace(c) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		for _, w := range watchedCols {
			pos := colPos[strings.ToLower(w)]
			if pos < 0 {
				continue // такой колонки нет — ок
			}
			val := ""
			if pos < len(rec) {
				val = strings.TrimSpace(rec[pos])
			}
			// строго только "yes" или "no"
			if val != "yes" && val != "no" {
				invalids = append(invalids, bad{colName: w, val: val, rowNum: rowNum})
			}
		}
	}

	if len(invalids) == 0 {
		return checks.ValidationResult{OK: true, Msg: "all flag columns contain only yes/no"}
	}

	limit := 10
	if len(invalids) < limit {
		limit = len(invalids)
	}
	var b strings.Builder
	b.WriteString("invalid values in flag columns: ")
	for i := 0; i < limit; i++ {
		inv := invalids[i]
		b.WriteString(inv.colName)
		b.WriteString(`="`)
		b.WriteString(inv.val)
		b.WriteString(`" (row `)
		b.WriteString(strconv.Itoa(inv.rowNum))
		b.WriteString(`)`)
		if i != limit-1 {
			b.WriteString("; ")
		}
	}
	if len(invalids) > limit {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(invalids)))
		b.WriteString(" invalid values)")
	} else {
		b.WriteString(" (total ")
		b.WriteString(strconv.Itoa(len(invalids)))
		b.WriteString(" invalid values)")
	}

	return checks.ValidationResult{OK: false, Msg: b.String()}
}
