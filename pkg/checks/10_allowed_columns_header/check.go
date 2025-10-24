package allowed_columns_header

import (
	"context"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-allowed-columns-header"

// поля, которые мы всегда считаем допустимыми системными
var coreAllowed = map[string]struct{}{
	"term":          {},
	"description":   {},
	"casesensitive": {},
	"translatable":  {},
	"forbidden":     {},
	"tags":          {},
}

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureAllowedColumnsHeader,
		checks.WithFailFast(),
		checks.WithPriority(10),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

// runEnsureAllowedColumnsHeader теперь просто заворачивает validate в RunWithFix.
// фикса у нас нет, поэтому Fix = nil и ShouldAttemptFix вернёт ErrNoFix,
// RunWithFix в этом случае вернёт FailAs как статус и не будет менять артефакт.
func runEnsureAllowedColumnsHeader(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:     checkName,
		Validate: validateAllowedColumnsHeader,
		Fix:      nil,

		// все не-ок сценарии у нас считаются не критическими:
		// - неизвестные колонки
		// - язык не из списка
		// - язык из списка отсутствует
		// это всё полезно подсветить, но не надо падать жёстко
		FailAs: checks.Warn,

		PassMsg:          "header columns are allowed",
		AppliedMsg:       "auto-fix applied", // не будет использоваться, Fix=nil, но поле нужно
		StillBadMsg:      "header columns have issues",
		StatusAfterFixed: checks.Pass, // не используется тут, но пусть будет консистентно
	})
}

// validateAllowedColumnsHeader реализует всю аналитику и сводит её к OK / not OK.
// важный момент: мы не проверяем порядок term/description — это уже сделано раньше.
func validateAllowedColumnsHeader(ctx context.Context, a checks.Artifact) checks.ValidationResult {
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
			Msg: "cannot check header: no usable content",
		}
	}

	lines := strings.Split(raw, "\n")
	headerIdx := firstNonEmptyLineIndex(lines)
	if headerIdx < 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot check header: no usable content",
		}
	}

	header := lines[headerIdx]
	cols := splitHeaderCells(header)

	// нормализованный в нижний регистр вайтлист языков, если юзер его передал
	allowedLangsNorm := map[string]struct{}{}
	for _, l := range a.Langs {
		allowedLangsNorm[strings.ToLower(strings.TrimSpace(l))] = struct{}{}
	}
	hasAllowed := len(allowedLangsNorm) > 0

	var unknownCols []string           // это вообще левая хрень (=> считаем проблемой)
	var unexpectedLangs []string       // язык в хедере, которого нет в a.Langs (=> warning case)
	var missingLangs []string          // язык из a.Langs, которого нет в хедере (=> warning case)
	var detectedLangsNoConfig []string // языки, которые мы угадали без конфига (=> soft warning)
	seenLang := map[string]bool{}      // lang, реально увиденный, только если он был в whitelist

	for _, col := range cols {
		colTrim := strings.TrimSpace(col)
		if colTrim == "" {
			continue
		}

		colLower := strings.ToLower(colTrim)

		// шаг 1: это одна из известных системных колонок? (term/description/...etc)
		if _, ok := coreAllowed[colLower]; ok {
			continue
		}

		// шаг 2: это похоже на языковую колонку?
		langBase, isLangLike := parseLangColumn(colTrim)

		if hasAllowed {
			// строгий режим (юзер дал список языков)

			if isLangLike {
				langKeyNorm := strings.ToLower(langBase)

				if _, ok := allowedLangsNorm[langKeyNorm]; ok {
					// нормальный язык из whitelist
					seenLang[langKeyNorm] = true
					continue
				}

				// выглядит как язык, но его нет в a.Langs
				unexpectedLangs = appendIfMissing(unexpectedLangs, langBase)
				continue
			}

			// не системная, не язык → мусор
			unknownCols = appendIfMissing(unknownCols, colTrim)

		} else {
			// свободный режим (нам не сказали список языков)

			if isLangLike {
				// мы такие: "языки вижу, но не с чем сравнивать"
				detectedLangsNoConfig = appendIfMissing(detectedLangsNoConfig, langBase)
				continue
			}

			// не язык и не системное поле → мусор
			unknownCols = appendIfMissing(unknownCols, colTrim)
		}
	}

	// строгий режим: проверяем что все объявленные языки реально есть
	if hasAllowed {
		for langNorm := range allowedLangsNorm {
			if !seenLang[langNorm] {
				missingLangs = appendIfMissing(missingLangs, langNorm)
			}
		}
	}

	// теперь сводим всё к одному результата:
	// правило приоритета сообщений:
	// 1. unknownCols → самое грязное, репортим в первую очередь
	// 2. потом unexpected/missing
	// 3. потом "я видел языки, но списка не дали"
	// 4. иначе всё ок

	if len(unknownCols) > 0 {
		return checks.ValidationResult{
			OK:  false,
			Msg: "header has unknown columns: " + strings.Join(unknownCols, ", "),
		}
	}

	if hasAllowed && (len(unexpectedLangs) > 0 || len(missingLangs) > 0) {
		var parts []string
		if len(unexpectedLangs) > 0 {
			parts = append(parts,
				"header has columns for undeclared languages: "+strings.Join(unexpectedLangs, ", "),
			)
		}
		if len(missingLangs) > 0 {
			parts = append(parts,
				"header is missing columns for declared languages: "+strings.Join(missingLangs, ", "),
			)
		}

		return checks.ValidationResult{
			OK:  false,
			Msg: strings.Join(parts, " ; "),
		}
	}

	if !hasAllowed && len(detectedLangsNoConfig) > 0 {
		return checks.ValidationResult{
			OK: false,
			Msg: "detected possible language columns: " +
				strings.Join(detectedLangsNoConfig, ", ") +
				"; no language list provided for validation",
		}
	}

	return checks.ValidationResult{
		OK:  true,
		Msg: "header columns are allowed",
	}
}

// parseLangColumn говорит: это "<code>" или "<code>_description"?
// возвращаем:
//
//	langBase (без "_description")
//	isLangLike (true/false)
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

// looksLikeLangCode пытается угадать локаль.
// допускаем:
//
//	en
//	fr
//	de
//	en_US
//	pt-BR
//	zh_Hans_CN
//
// и т.д.
//
// логика:
// - заменяем "-" на "_" для анализа
// - первая часть (до "_") должна быть 2..3 буквы A-Za-z
// - остальные части (если есть) состоят из [A-Za-z0-9]+
//
// это покрывает короткие коды, регионы, вариации скрипта и т.д.
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
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}

	for _, seg := range parts[1:] {
		if seg == "" {
			return false
		}
		for _, r := range seg {
			if !(r >= 'a' && r <= 'z' ||
				r >= 'A' && r <= 'Z' ||
				r >= '0' && r <= '9') {
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

// splitHeaderCells: режем по ';' и тримаем по краям.
// к этому моменту предыдущие фиксы уже должны были подчистить пробелы в хедере,
// так что TrimSpace тут не ломает инварианты.
func splitHeaderCells(header string) []string {
	rawCells := strings.Split(header, ";")
	out := make([]string, 0, len(rawCells))
	for _, c := range rawCells {
		out = append(out, strings.TrimSpace(c))
	}
	return out
}

func firstNonEmptyLineIndex(lines []string) int {
	for i, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			return i
		}
	}
	return -1
}
