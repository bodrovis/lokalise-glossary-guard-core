package term_description_header

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-term-description-header"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureTermDescriptionHeader,
		checks.WithFailFast(),
		checks.WithPriority(9),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runEnsureTermDescriptionHeader(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateTermDescriptionHeader,
		Fix:              fixTermDescriptionHeader,
		PassMsg:          "header starts with term;description",
		FixedMsg:         "normalized header to start with term;description",
		AppliedMsg:       "auto-fix applied: normalized header to start with term;description",
		StatusAfterFixed: checks.Pass,
		StillBadMsg:      "header still does not start with term;description after fix",
	})
}

func validateTermDescriptionHeader(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}
	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: false, Msg: "cannot check header: no usable content"}
	}

	// читаем первую непустую CSV-запись как заголовок
	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var header []string
	for {
		rec, err := r.Read()
		if err != nil {
			if ctx.Err() != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
			}
			return checks.ValidationResult{OK: false, Msg: "cannot parse header with semicolon delimiter", Err: err}
		}
		// проверяем «непустую» запись
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

	if len(header) < 2 {
		return checks.ValidationResult{OK: false, Msg: "header has fewer than two columns; expected at least term;description"}
	}

	first := strings.ToLower(strings.TrimSpace(header[0]))
	second := strings.ToLower(strings.TrimSpace(header[1]))
	if first == "term" && second == "description" {
		return checks.ValidationResult{OK: true, Msg: "header starts with term;description"}
	}

	hasTerm, hasDesc := false, false
	for _, c := range header {
		cc := strings.ToLower(strings.TrimSpace(c))
		switch cc {
		case "term":
			hasTerm = true
		case "description":
			hasDesc = true
		}
	}

	switch {
	case hasTerm && hasDesc:
		return checks.ValidationResult{OK: false, Msg: "header contains term and description but not in required order or not at the start"}
	case hasTerm && !hasDesc:
		return checks.ValidationResult{OK: false, Msg: "header contains term but missing description column"}
	case !hasTerm && hasDesc:
		return checks.ValidationResult{OK: false, Msg: "header contains description but missing term column"}
	default:
		return checks.ValidationResult{OK: false, Msg: "header missing both term and description columns"}
	}
}
