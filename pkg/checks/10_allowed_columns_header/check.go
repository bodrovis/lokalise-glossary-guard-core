package allowed_columns_header

import (
	"context"
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
		return cancelledValidation(err)
	}

	data := checks.StripUTF8BOM(a.Data)
	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot check header: no usable content",
		}
	}

	header, res, ok := readAllowedColumnsHeader(ctx, data)
	if !ok {
		return res
	}

	report, err := inspectAllowedColumns(ctx, header, a.Langs)
	if err != nil {
		return cancelledValidation(err)
	}

	return allowedColumnsValidationResult(report)
}

func readAllowedColumnsHeader(
	ctx context.Context,
	data []byte,
) ([]string, checks.ValidationResult, bool) {
	r := checks.NewSemicolonCSVReader(data)

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

func isBlankCSVRecord(record []string) bool {
	for _, col := range record {
		if !checks.IsBlankUnicode([]byte(col)) {
			return false
		}
	}

	return true
}

type allowedColumnsReport struct {
	hasAllowedConfig      bool
	unknownCols           []string
	unexpectedLangs       []string
	missingLangColumns    []string
	detectedLangsNoConfig []string
}

type allowedLanguages struct {
	keys []string
	set  map[string]string
}

func newAllowedLanguages(langs []string) allowedLanguages {
	out := allowedLanguages{
		set: make(map[string]string, len(langs)),
	}

	for _, lang := range langs {
		key := normalizeLangKey(lang)
		if key == "" {
			continue
		}

		if _, exists := out.set[key]; exists {
			continue
		}

		out.keys = append(out.keys, key)
		out.set[key] = key
	}

	return out
}

func (l allowedLanguages) hasAny() bool {
	return len(l.keys) > 0
}

type languagePresence struct {
	value       bool
	description bool
}

func inspectAllowedColumns(
	ctx context.Context,
	cols []string,
	langs []string,
) (allowedColumnsReport, error) {
	allowed := newAllowedLanguages(langs)

	report := allowedColumnsReport{
		hasAllowedConfig: allowed.hasAny(),
	}

	seen := make(map[string]languagePresence, len(allowed.keys))

	for _, col := range cols {
		if err := ctx.Err(); err != nil {
			return allowedColumnsReport{}, err
		}

		inspectAllowedColumn(&report, seen, allowed, col)
	}

	if allowed.hasAny() {
		report.missingLangColumns = missingDeclaredLanguageColumns(allowed, seen)
	}

	return report, nil
}

func inspectAllowedColumn(
	report *allowedColumnsReport,
	seen map[string]languagePresence,
	allowed allowedLanguages,
	col string,
) {
	colTrim := strings.TrimSpace(col)
	if colTrim == "" {
		return
	}

	colLower := strings.ToLower(colTrim)
	if _, ok := checks.KnownHeaders[colLower]; ok {
		return
	}

	langCol, isLangLike := parseLangColumn(colTrim)

	if allowed.hasAny() {
		if isLangLike {
			if _, ok := allowed.set[langCol.key]; ok {
				p := seen[langCol.key]
				if langCol.description {
					p.description = true
				} else {
					p.value = true
				}
				seen[langCol.key] = p
				return
			}

			report.unexpectedLangs = appendLangIfMissing(report.unexpectedLangs, langCol.base)
			return
		}

		report.unknownCols = appendStringIfMissingFold(report.unknownCols, colTrim)
		return
	}

	if isLangLike {
		report.detectedLangsNoConfig = appendLangIfMissing(report.detectedLangsNoConfig, langCol.base)
		return
	}

	report.unknownCols = appendStringIfMissingFold(report.unknownCols, colTrim)
}

type parsedLangColumn struct {
	base        string
	key         string
	description bool
}

func parseLangColumn(col string) (parsedLangColumn, bool) {
	col = strings.TrimSpace(col)
	if col == "" {
		return parsedLangColumn{}, false
	}

	colLower := strings.ToLower(col)

	if strings.HasSuffix(colLower, "_description") {
		base := col[:len(col)-len("_description")]
		if !looksLikeLangCode(base) {
			return parsedLangColumn{}, false
		}

		return parsedLangColumn{
			base:        base,
			key:         normalizeLangKey(base),
			description: true,
		}, true
	}

	if looksLikeLangCode(col) {
		return parsedLangColumn{
			base: col,
			key:  normalizeLangKey(col),
		}, true
	}

	return parsedLangColumn{}, false
}

func normalizeLangKey(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return ""
	}

	lang = strings.ReplaceAll(lang, "-", "_")
	return strings.ToLower(lang)
}

func missingDeclaredLanguageColumns(
	allowed allowedLanguages,
	seen map[string]languagePresence,
) []string {
	var missing []string

	for _, key := range allowed.keys {
		p := seen[key]

		if !p.value {
			missing = append(missing, key)
		}

		if !p.description {
			missing = append(missing, key+"_description")
		}
	}

	return missing
}

func allowedColumnsValidationResult(report allowedColumnsReport) checks.ValidationResult {
	if len(report.unknownCols) > 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "header has unknown columns: " + strings.Join(report.unknownCols, ", "),
		}
	}

	if report.hasAllowedConfig &&
		(len(report.unexpectedLangs) > 0 || len(report.missingLangColumns) > 0) {
		var parts []string

		if len(report.unexpectedLangs) > 0 {
			parts = append(parts,
				"header has columns for undeclared languages: "+strings.Join(report.unexpectedLangs, ", "),
			)
		}

		if len(report.missingLangColumns) > 0 {
			parts = append(parts,
				"header is missing columns for declared languages: "+strings.Join(report.missingLangColumns, ", "),
			)
		}

		return checks.ValidationResult{
			OK:  false,
			Msg: strings.Join(parts, " ; "),
		}
	}

	if !report.hasAllowedConfig && len(report.detectedLangsNoConfig) > 0 {
		return checks.ValidationResult{
			OK: true,
			Msg: "header columns look like languages: " + strings.Join(report.detectedLangsNoConfig, ", ") +
				" (no declared language list, skipped strict validation)",
		}
	}

	return checks.ValidationResult{
		OK:  true,
		Msg: "header columns are allowed",
	}
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}

func appendLangIfMissing(sl []string, lang string) []string {
	key := normalizeLangKey(lang)

	for _, existing := range sl {
		if normalizeLangKey(existing) == key {
			return sl
		}
	}

	return append(sl, lang)
}

func appendStringIfMissingFold(sl []string, v string) []string {
	vLower := strings.ToLower(v)

	for _, existing := range sl {
		if strings.ToLower(existing) == vLower {
			return sl
		}
	}

	return append(sl, v)
}

func looksLikeLangCode(s string) bool {
	if s == "" {
		return false
	}

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
		if !isASCIILetter(r) {
			return false
		}
	}

	for _, seg := range parts[1:] {
		if seg == "" {
			return false
		}

		for _, r := range seg {
			if !isASCIILetter(r) && !isASCIIDigit(r) {
				return false
			}
		}
	}

	return true
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
