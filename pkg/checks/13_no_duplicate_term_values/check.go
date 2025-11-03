package duplicate_term_values

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "warn-duplicate-term-values"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runWarnDuplicateTermValues,
		// no FailFast(): duplicates in term are bad but not blocker
		checks.WithPriority(13),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

// runWarnDuplicateTermValues wires validation only. No auto-fix yet.
func runWarnDuplicateTermValues(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateWarnDuplicateTermValues,
		Fix:              fixDuplicateTermValues,
		PassMsg:          "no duplicate term values",
		FixedMsg:         "removed duplicate term rows",
		AppliedMsg:       "auto-fix applied: removed duplicate term rows",
		StatusAfterFixed: checks.Pass,
		FailAs:           checks.Warn,
		StillBadMsg:      "duplicate term values are still present after fix",
	})
}

// validateWarnDuplicateTermValues scans the "term" column and warns if the same non-empty value appears in multiple rows.
// Case-sensitive: "Apple" and "apple" are considered different terms.
// We report up to 10 offending term groups in the message, each annotated with row numbers (1-based).
func validateWarnDuplicateTermValues(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: true, Msg: "no content to validate for duplicate term values"}
	}

	// CSV reader с разделителем ';'
	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	// читаем первую непустую запись как хедер
	var header []string
	recIdx := 0 // 1-based номер текущей записи
	for {
		rec, err := r.Read()
		if err != nil {
			if ctx.Err() != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
			}
			return checks.ValidationResult{OK: true, Msg: "no header line found (nothing to validate for duplicate term values)"}
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

	// индекс колонки term (case-insensitive)
	termCol := -1
	for i, h := range header {
		if strings.EqualFold(strings.TrimSpace(h), "term") {
			termCol = i
			break
		}
	}
	if termCol < 0 {
		return checks.ValidationResult{OK: true, Msg: "no 'term' column found (skipping duplicate term check)"}
	}

	// term -> список номеров строк (1-based по CSV-записям)
	seen := make(map[string][]int)

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
			break // EOF или парс-ошибка — другие чеки разрулят
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

		val := ""
		if termCol < len(rec) {
			val = strings.TrimSpace(rec[termCol])
		}
		if val == "" {
			continue // пустые термы валидируются отдельным чеком
		}

		// важное: дубликаты считаем КЕЙС-СЕНСИТИВНО
		seen[val] = append(seen[val], rowNum)
	}

	// собираем группы с >=2 вхождениями
	type dupInfo struct {
		term string
		rows []int
	}
	var dups []dupInfo
	for term, rows := range seen {
		if len(rows) >= 2 {
			dups = append(dups, dupInfo{term: term, rows: rows})
		}
	}
	if len(dups) == 0 {
		return checks.ValidationResult{OK: true, Msg: "no duplicate term values"}
	}

	// форматируем до 10 групп
	limit := 10
	if len(dups) < limit {
		limit = len(dups)
	}
	var b strings.Builder
	b.WriteString("duplicate term values found: ")
	for i := 0; i < limit; i++ {
		d := dups[i]
		b.WriteString(`"`)
		b.WriteString(d.term)
		b.WriteString(`" (rows `)
		b.WriteString(joinIntSlice(d.rows, ", "))
		b.WriteString(`)`)
		if i != limit-1 {
			b.WriteString("; ")
		}
	}
	if len(dups) > limit {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(dups)))
		b.WriteString(" duplicate terms)")
	} else {
		b.WriteString(" (total ")
		b.WriteString(strconv.Itoa(len(dups)))
		b.WriteString(" duplicate terms)")
	}

	return checks.ValidationResult{OK: false, Msg: b.String()}
}

// joinIntSlice joins ints like "2, 5, 10".
func joinIntSlice(nums []int, sep string) string {
	if len(nums) == 0 {
		return ""
	}
	if len(nums) == 1 {
		return strconv.Itoa(nums[0])
	}
	var b strings.Builder
	b.WriteString(strconv.Itoa(nums[0]))
	for i := 1; i < len(nums); i++ {
		b.WriteString(sep)
		b.WriteString(" ")
		b.WriteString(strconv.Itoa(nums[i]))
	}
	return b.String()
}
