package no_empty_term_values

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-empty-term-values"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoEmptyTermValues,
		checks.WithFailFast(),
		checks.WithPriority(12),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoEmptyTermValues(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:     checkName,
		Validate: validateNoEmptyTermValues,
		Fix:      nil,
		PassMsg:  "all rows have non-empty term",
	})
}

func validateNoEmptyTermValues(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: true, Msg: "no content to validate for empty term values"}
	}

	// CSV reader c разделителем ';'
	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	// читаем первую НЕПУСТУЮ запись как хедер
	var header []string
	recIdx := 0 // 1-based индекс записи для сообщений
	for {
		rec, err := r.Read()
		if err != nil {
			if ctx.Err() != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
			}
			// нет записей — нечего валидировать
			return checks.ValidationResult{OK: true, Msg: "no header line found (nothing to validate for empty term values)"}
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

	// ищем колонку "term" (case-insensitive)
	termCol := -1
	for i, h := range header {
		if strings.ToLower(strings.TrimSpace(h)) == "term" {
			termCol = i
			break
		}
	}
	if termCol < 0 {
		return checks.ValidationResult{OK: true, Msg: "no 'term' column found (skipping empty term validation)"}
	}

	// проверяем остальные записи
	var badRows []int
	rowNum := recIdx // текущий 1-based номер записи (после чтения хедера)
	const checkEvery = 1 << 12
	for {
		if (rowNum % checkEvery) == 0 {
			if err := ctx.Err(); err != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
			}
		}

		rec, err := r.Read()
		if err != nil {
			break // EOF или ошибка — другие чеки отрепортят парсинг
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
			badRows = append(badRows, rowNum) // уже 1-based
		}
	}

	if len(badRows) == 0 {
		return checks.ValidationResult{OK: true, Msg: "all rows have non-empty term"}
	}

	display := badRows
	if len(display) > 10 {
		display = display[:10]
	}
	var b strings.Builder
	b.WriteString("empty term in rows: ")
	for i, n := range display {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Itoa(n))
	}
	if len(badRows) > 10 {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(badRows)))
		b.WriteString(")")
	} else {
		b.WriteString(" (total ")
		b.WriteString(strconv.Itoa(len(badRows)))
		b.WriteString(")")
	}

	return checks.ValidationResult{OK: false, Msg: b.String()}
}
