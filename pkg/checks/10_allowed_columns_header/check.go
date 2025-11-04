package allowed_columns_header

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-allowed-columns-header"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureAllowedColumnsHeader,
		checks.WithPriority(10),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runEnsureAllowedColumnsHeader(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateAllowedColumnsHeader,
		Fix:              fixAllowedColumnsHeader,
		FailAs:           checks.Warn,
		PassMsg:          "header columns are allowed",
		FixedMsg:         "header columns normalized (unknown columns removed, missing language columns added)",
		AppliedMsg:       "auto-fix applied to header columns",
		StillBadMsg:      "header columns still have issues after auto-fix",
		StatusAfterFixed: checks.Pass,
	})
}

func validateAllowedColumnsHeader(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: false, Msg: "cannot check header: no usable content"}
	}

	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var cols []string
	for {
		rec, err := r.Read()
		if err != nil || rec == nil {
			if ctx.Err() != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
			}
			return checks.ValidationResult{OK: false, Msg: "cannot parse header with semicolon delimiter", Err: err}
		}
		nonEmpty := false
		for _, c := range rec {
			if strings.TrimSpace(c) != "" {
				nonEmpty = true
				break
			}
		}
		if nonEmpty {
			cols = rec
			break
		}
	}

	allowedLangsNorm := map[string]struct{}{}
	for _, l := range a.Langs {
		allowedLangsNorm[strings.ToLower(strings.TrimSpace(l))] = struct{}{}
	}
	hasAllowed := len(allowedLangsNorm) > 0

	var unknownCols []string
	var unexpectedLangs []string
	var missingLangs []string
	var detectedLangsNoConfig []string
	seenLang := map[string]bool{}

	for _, col := range cols {
		if err := ctx.Err(); err != nil {
			return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
		}
		colTrim := strings.TrimSpace(col)
		if colTrim == "" {
			continue
		}
		colLower := strings.ToLower(colTrim)

		if _, ok := checks.KnownHeaders[colLower]; ok {
			continue
		}

		langBase, isLangLike := parseLangColumn(colTrim)

		if hasAllowed {
			if isLangLike {
				langKeyNorm := strings.ToLower(langBase)
				if _, ok := allowedLangsNorm[langKeyNorm]; ok {
					seenLang[langKeyNorm] = true
					continue
				}
				unexpectedLangs = appendIfMissing(unexpectedLangs, langBase)
				continue
			}
			unknownCols = appendIfMissing(unknownCols, colTrim)
		} else {
			if isLangLike {
				detectedLangsNoConfig = appendIfMissing(detectedLangsNoConfig, langBase)
				continue
			}
			unknownCols = appendIfMissing(unknownCols, colTrim)
		}
	}

	if hasAllowed {
		for langNorm := range allowedLangsNorm {
			if !seenLang[langNorm] {
				missingLangs = appendIfMissing(missingLangs, langNorm)
			}
		}
	}

	if len(unknownCols) > 0 {
		return checks.ValidationResult{OK: false, Msg: "header has unknown columns: " + strings.Join(unknownCols, ", ")}
	}
	if hasAllowed && (len(unexpectedLangs) > 0 || len(missingLangs) > 0) {
		var parts []string
		if len(unexpectedLangs) > 0 {
			parts = append(parts, "header has columns for undeclared languages: "+strings.Join(unexpectedLangs, ", "))
		}
		if len(missingLangs) > 0 {
			parts = append(parts, "header is missing columns for declared languages: "+strings.Join(missingLangs, ", "))
		}
		return checks.ValidationResult{OK: false, Msg: strings.Join(parts, " ; ")}
	}
	if !hasAllowed && len(detectedLangsNoConfig) > 0 {
		return checks.ValidationResult{
			OK: true,
			Msg: "header columns look like languages: " + strings.Join(detectedLangsNoConfig, ", ") +
				" (no declared language list, skipped strict validation)",
		}
	}

	return checks.ValidationResult{OK: true, Msg: "header columns are allowed"}
}

func parseLangColumn(col string) (langBase string, isLangLike bool) {
	if strings.HasSuffix(col, "_description") {
		base := strings.TrimSuffix(col, "_description")
		if looksLikeLangCode(base) {
			return base, true
		}
		return "", false
	}

	if looksLikeLangCode(col) {
		return col, true
	}

	return "", false
}

// Supported locales:
//
//	en
//	fr
//	de
//	en_US
//	pt-BR
//	zh_Hans_CN
//
// and so on.
func looksLikeLangCode(s string) bool {
	if s == "" {
		return false
	}
	// treat "-" same as "_"
	s = strings.ReplaceAll(s, "-", "_")

	parts := strings.Split(s, "_")
	if len(parts) == 0 {
		return false
	}

	first := parts[0]
	if len(first) < 2 || len(first) > 3 {
		return false
	}
	for _, r := range first {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}

	for _, seg := range parts[1:] {
		if seg == "" {
			return false
		}
		for _, r := range seg {
			if (r < 'a' || r > 'z') &&
				(r < 'A' || r > 'Z') &&
				(r < '0' || r > '9') {
				return false
			}
		}
	}

	return true
}

func appendIfMissing(sl []string, v string) []string {
	vLower := strings.ToLower(v)
	for _, x := range sl {
		if strings.ToLower(x) == vLower {
			return sl
		}
	}
	return append(sl, v)
}
