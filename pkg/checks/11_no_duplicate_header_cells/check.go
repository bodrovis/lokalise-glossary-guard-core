package duplicate_header_cells

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

const checkName = "warn-duplicate-header-cells"

func init() {
	ch, err := checks.NewCheckAdapter(
		checkName,
		runWarnDuplicateHeaderCells,
		checks.WithPriority(11),
	)
	if err != nil {
		panic(checkName + ": " + err.Error())
	}
	if _, err := checks.Register(ch); err != nil {
		panic(checkName + " register: " + err.Error())
	}
}

func runWarnDuplicateHeaderCells(ctx context.Context, a checks.Artifact, opts checks.RunOptions) checks.CheckOutcome {
	return checks.RunWithFix(ctx, a, opts, checks.RunRecipe{
		Name:             checkName,
		Validate:         validateDuplicateHeaderCells,
		Fix:              fixDuplicateHeaderCells,
		PassMsg:          "no duplicate header columns",
		FixedMsg:         "removed duplicate header columns",
		AppliedMsg:       "auto-fix applied: removed duplicate header columns",
		StatusAfterFixed: checks.Pass,
		FailAs:           checks.Warn,
		StillBadMsg:      "header still contains duplicate columns after fix",
	})
}

func validateDuplicateHeaderCells(ctx context.Context, a checks.Artifact) checks.ValidationResult {
	if err := ctx.Err(); err != nil {
		return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
	}

	if len(bytes.TrimSpace(a.Data)) == 0 {
		return checks.ValidationResult{OK: true, Msg: "no content to check for duplicate headers"}
	}

	// читаем первую НЕПУСТУЮ запись как заголовок
	br := bufio.NewReader(bytes.NewReader(a.Data))
	r := csv.NewReader(br)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var header []string
	for {
		rec, err := r.Read()
		if err != nil || rec == nil {
			if ctx.Err() != nil {
				return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: ctx.Err()}
			}
			// не смогли распарсить — пусть другие чеки рулят
			return checks.ValidationResult{OK: true, Msg: "no header line found (nothing to check for duplicates)"}
		}
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

	type stat struct {
		Count  int
		Sample string
	}
	seen := make(map[string]*stat)

	for _, c := range header {
		if err := ctx.Err(); err != nil {
			return checks.ValidationResult{OK: false, Msg: "validation cancelled", Err: err}
		}
		trimmed := strings.TrimSpace(c)
		// ключом считаем тримнутую строку (включая пустую!)
		key := strings.ToLower(trimmed)

		// красивое имя для репорта
		sample := trimmed
		if sample == "" {
			sample = `"<empty>"`
		}

		if s, ok := seen[key]; ok {
			s.Count++
		} else {
			seen[key] = &stat{Count: 1, Sample: sample}
		}
	}

	var dups []string
	for _, st := range seen {
		if st.Count > 1 {
			dups = append(dups, st.Sample+"("+strconv.Itoa(st.Count)+")")
		}
	}

	if len(dups) == 0 {
		return checks.ValidationResult{OK: true, Msg: "no duplicate header columns"}
	}
	return checks.ValidationResult{OK: false, Msg: "duplicate header columns: " + strings.Join(dups, ", ")}
}
