package semicolon_separator

import (
	"context"
	"encoding/csv"
	"io"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "ensure-semicolon-separators"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runEnsureSemicolonSeparators,
		checks.WithFailFast(),
		checks.WithPriority(6),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runEnsureSemicolonSeparators(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateSemicolonSeparated,
		Fix:              fixToSemicolonsIfConsistent,
		PassMsg:          "file uses semicolons as separators",
		FixedMsg:         "converted separators to semicolons",
		AppliedMsg:       "auto-fix applied: converted separators to semicolons",
		StatusAfterFixed: checks.Pass,
	})
}

// validateSemicolonSeparated confirms the file is consistently delimited by ';'.
// If it's consistently delimited by other common delimiters, it fails (so Fix can kick in).
// If nothing looks consistent, it fails generically (no fix).
func validateSemicolonSeparated(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{
			OK:  false,
			Msg: "validation cancelled",
			Err: err,
		}
	}

	data := strings.TrimSpace(string(a.Data))
	if data == "" {
		return checks.ValidationResult{
			OK:  false,
			Msg: "cannot detect separators: no usable content",
		}
	}

	const maxLines = 100

	// 1. Happy path: already looks like proper semicolon-separated CSV.
	if ok, _ := looksLikeDelimited(data, ';', maxLines); ok {
		return checks.ValidationResult{
			OK: true,
		}
	}

	// 2. Check if it's consistently comma-separated or tab-separated.
	commaOK, _ := looksLikeDelimited(data, ',', maxLines)
	tabOK, _ := looksLikeDelimited(data, '\t', maxLines)

	// 3. Only if it's neither proper commas nor proper tabs,
	//    try to classify it as "mixed delimiter soup".
	//
	//    Why: A valid comma-separated CSV may still contain semicolons
	//    inside quoted fields ("foo;bar"), which is fine and fixable.
	//    We don't want to block auto-fix in that case.
	if !commaOK && !tabOK && hasMixedDelimiters(data) {
		return checks.ValidationResult{
			OK:  false,
			Msg: "inconsistent/mixed delimiters detected (e.g. both ',' and ';'); cannot safely auto-convert; expected semicolons (;)",
		}
	}

	// 4. Stable comma-separated CSV (fixable).
	if commaOK {
		return checks.ValidationResult{
			OK:  false,
			Msg: "file appears to use commas as separators; expected semicolons (;)",
		}
	}

	// 5. Stable tab-separated TSV (also fixable).
	if tabOK {
		return checks.ValidationResult{
			OK:  false,
			Msg: "file appears to use tabs as separators; expected semicolons (;)",
		}
	}

	// 6. Totally unclear / structurally inconsistent.
	return checks.ValidationResult{
		OK:  false,
		Msg: "could not confirm a consistent semicolon-separated format; expected semicolons (;)",
	}
}

// looksLikeDelimited tries to parse up to N records using a given delimiter and
// decides if the file is "consistently delimited":
//   - parse succeeds for at least 2 non-empty records,
//   - each record has >=2 fields,
//   - field count is stable (allowing at most 1 mismatch to be tolerant to odd headers).
func looksLikeDelimited(data string, delim rune, max int) (bool, int) {
	r := csv.NewReader(strings.NewReader(data))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	read := 0
	stableCols := -1
	mismatch := 0

	for {
		if read >= max {
			break
		}
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, 0
		}

		nonEmpty := 0
		for _, f := range rec {
			if strings.TrimSpace(f) != "" {
				nonEmpty++
			}
		}
		if nonEmpty == 0 {
			continue
		}

		read++
		fields := len(rec)
		if fields < 2 {
			return false, 0
		}

		if stableCols < 0 {
			stableCols = fields
		} else if fields != stableCols {
			mismatch++
			if mismatch > 1 {
				return false, 0
			}
		}
	}

	if read < 2 || stableCols < 2 {
		return false, 0
	}
	return true, stableCols
}

func hasMixedDelimiters(data string) bool {
	lines := splitLinesLimited(data, 20)

	type sig struct {
		semi  int
		comma int
		tab   int
	}

	lineSigs := make([]sig, 0, len(lines))

	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}

		s := sig{
			semi:  strings.Count(trimmed, ";"),
			comma: strings.Count(trimmed, ","),
			tab:   strings.Count(trimmed, "\t"),
		}

		kindCount := 0
		if s.semi > 0 {
			kindCount++
		}
		if s.comma > 0 {
			kindCount++
		}
		if s.tab > 0 {
			kindCount++
		}
		if kindCount > 1 {
			return true
		}

		lineSigs = append(lineSigs, s)
	}

	var prevMain rune = 0
	for _, s := range lineSigs {
		curMain := rune(0)

		if s.semi >= 2 && s.comma == 0 && s.tab == 0 {
			curMain = ';'
		} else if s.comma >= 2 && s.semi == 0 && s.tab == 0 {
			curMain = ','
		} else if s.tab >= 2 && s.semi == 0 && s.comma == 0 {
			curMain = '\t'
		}

		if curMain == 0 {
			continue
		}
		if prevMain == 0 {
			prevMain = curMain
			continue
		}
		if curMain != prevMain {
			return true
		}
	}

	return false
}

func splitLinesLimited(s string, limit int) []string {
	out := make([]string, 0, limit)
	start := 0
	for start < len(s) && len(out) < limit {
		i := strings.IndexByte(s[start:], '\n')
		if i < 0 {
			out = append(out, s[start:])
			break
		}
		out = append(out, s[start:start+i])
		start += i + 1
	}
	return out
}
