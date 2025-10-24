package no_spaces_in_header

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "no-spaces-in-header"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runNoSpacesInHeader,
		checks.WithFailFast(),
		checks.WithPriority(8),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runNoSpacesInHeader(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateNoSpacesInHeader,
		Fix:              fixNoSpacesInHeader,
		FailAs:           checks.Warn,
		PassMsg:          "header columns are trimmed (no leading/trailing spaces)",
		FixedMsg:         "header auto-fixed: trimmed leading/trailing spaces in column names",
		AppliedMsg:       "auto-fix applied to header",
		StillBadMsg:      "auto-fix attempted but header still invalid",
		StatusAfterFixed: checks.Pass,
	})
}

// validateNoSpacesInHeader проверяет только первую строку файла (header).
// считаем что разделитель уже гарантирован как ';', файл не пустой и т.д.
func validateNoSpacesInHeader(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{
			OK:  false,
			Msg: "validation cancelled",
			Err: err,
		}
	}

	raw := string(a.Data)
	if raw == "" {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot check header: empty content",
		}
	}

	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot check header: empty content",
		}
	}

	header := lines[0]

	// разбираем хедер по ; — считаем что ; верный и он уже проверен до этого чекера
	cells := strings.Split(header, ";")

	for _, c := range cells {
		if c != strings.TrimSpace(c) {
			// то есть есть лид/трейл пробелы
			return checks.ValidationResult{
				OK:  false,
				Msg: "header has leading/trailing spaces in column names",
			}
		}
	}

	return checks.ValidationResult{
		OK:  true,
		Msg: "header columns are trimmed (no leading/trailing spaces)",
	}
}
