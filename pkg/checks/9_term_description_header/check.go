package term_description_header

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-term-description-header"

type termDescriptionReport struct {
	ok      bool
	message string
}

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
	header, res, ok := readFirstNonBlankHeader(ctx, a)
	if !ok {
		return res
	}

	if len(header) < 2 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "header has fewer than two columns; expected at least term;description",
		}
	}

	report := inspectTermDescriptionHeader(header)
	if report.ok {
		return checks.ValidationResult{
			OK:  true,
			Msg: "header starts with term;description",
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: report.message,
	}
}

func readFirstNonBlankHeader(
	ctx context.Context,
	a checks.Artifact,
) ([]string, checks.ValidationResult, bool) {
	data := checks.StripUTF8BOM(a.Data)

	r, res, ok := checks.NewSemicolonCSVReaderWithCtx(
		ctx,
		checks.Artifact{
			Data:  data,
			Path:  a.Path,
			Langs: a.Langs,
		},
		"cannot check header: no usable content",
	)
	if !ok {
		return nil, res, false
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, cancelledValidation(err), false
		}

		rec, err := r.Read()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, cancelledValidation(ctxErr), false
			}

			return nil, checks.ValidationResult{
				OK:  false,
				Msg: "cannot parse header with semicolon delimiter",
				Err: err,
			}, false
		}

		if !isBlankCSVRecord(rec) {
			return rec, checks.ValidationResult{}, true
		}
	}
}

func inspectTermDescriptionHeader(header []string) termDescriptionReport {
	first := normalizeHeaderCell(header[0])
	second := normalizeHeaderCell(header[1])

	if first == "term" && second == "description" {
		return termDescriptionReport{ok: true}
	}

	hasTerm, hasDescription := hasRequiredHeaderColumns(header)

	switch {
	case hasTerm && hasDescription:
		return termDescriptionReport{
			message: "header contains term and description but not in required order or not at the start",
		}
	case hasTerm && !hasDescription:
		return termDescriptionReport{
			message: "header contains term but missing description column",
		}
	case !hasTerm && hasDescription:
		return termDescriptionReport{
			message: "header contains description but missing term column",
		}
	default:
		return termDescriptionReport{
			message: "header missing both term and description columns",
		}
	}
}

func hasRequiredHeaderColumns(header []string) (bool, bool) {
	hasTerm := false
	hasDescription := false

	for _, col := range header {
		switch normalizeHeaderCell(col) {
		case "term":
			hasTerm = true
		case "description":
			hasDescription = true
		}
	}

	return hasTerm, hasDescription
}

func normalizeHeaderCell(col string) string {
	return strings.ToLower(strings.TrimSpace(col))
}

func isBlankCSVRecord(record []string) bool {
	for _, col := range record {
		if !checks.IsBlankUnicode([]byte(col)) {
			return false
		}
	}

	return true
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
