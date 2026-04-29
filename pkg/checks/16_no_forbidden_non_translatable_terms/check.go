package no_forbidden_non_translatable_terms

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-forbidden-non-translatable-terms"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoForbiddenNonTranslatableTerms,
		checks.WithFailFast(),
		checks.WithPriority(16),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoForbiddenNonTranslatableTerms(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:     checkName,
		Validate: validateNoForbiddenNonTranslatableTerms,
		Fix:      nil,
		PassMsg:  "no forbidden non-translatable terms found",
	})
}

func validateNoForbiddenNonTranslatableTerms(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: true, Msg: "no content to validate for forbidden non-translatable terms"}
	}

	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var header []string
	recIdx := 0
	for {
		rec, err := r.Read()
		if err != nil {
			if ctx.Err() != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
			}
			return checks.ValidationResult{OK: true, Msg: "no header line found (nothing to validate for forbidden non-translatable terms)"}
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

	termCol := -1
	translatableCol := -1
	forbiddenCol := -1

	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "term":
			termCol = i
		case "translatable":
			translatableCol = i
		case "forbidden":
			forbiddenCol = i
		}
	}

	if translatableCol < 0 || forbiddenCol < 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "translatable or forbidden column not found (skipping forbidden non-translatable validation)",
		}
	}

	type bad struct {
		rowNum int
		term   string
	}

	var badRows []bad
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
			break
		}
		rowNum++

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

		translatable := ""
		if translatableCol < len(rec) {
			translatable = strings.TrimSpace(rec[translatableCol])
		}

		forbidden := ""
		if forbiddenCol < len(rec) {
			forbidden = strings.TrimSpace(rec[forbiddenCol])
		}

		// Forbidden + non-translatable means:
		//   translatable=no
		//   forbidden=yes
		if translatable == "no" && forbidden == "yes" {
			term := ""
			if termCol >= 0 && termCol < len(rec) {
				term = strings.TrimSpace(rec[termCol])
			}

			badRows = append(badRows, bad{
				rowNum: rowNum,
				term:   term,
			})
		}
	}

	if len(badRows) == 0 {
		return checks.ValidationResult{OK: true, Msg: "no forbidden non-translatable terms found"}
	}

	limit := min(len(badRows), 10)

	var b strings.Builder
	b.WriteString("terms cannot be both forbidden and non-translatable: ")

	for i := range limit {
		br := badRows[i]

		if br.term != "" {
			b.WriteString(`term="`)
			b.WriteString(br.term)
			b.WriteString(`" `)
		}

		b.WriteString("(row ")
		b.WriteString(strconv.Itoa(br.rowNum))
		b.WriteString(")")

		if i != limit-1 {
			b.WriteString("; ")
		}
	}

	if len(badRows) > limit {
		b.WriteString(" ... (total ")
		b.WriteString(strconv.Itoa(len(badRows)))
		b.WriteString(" terms)")
	} else {
		b.WriteString(" (total ")
		b.WriteString(strconv.Itoa(len(badRows)))
		b.WriteString(" terms)")
	}

	return checks.ValidationResult{OK: false, Msg: b.String()}
}
