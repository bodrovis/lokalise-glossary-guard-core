package duplicate_header_cells

import (
	"context"
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
		return cancelledValidation(err)
	}

	data := checks.StripUTF8BOM(a.Data)
	if checks.IsBlankUnicode(data) {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no content to check for duplicate headers",
		}
	}

	header, res, ok := readDuplicateHeader(ctx, data)
	if !ok {
		return res
	}

	dups, err := findDuplicateHeaderCells(ctx, header)
	if err != nil {
		return cancelledValidation(err)
	}

	if len(dups) == 0 {
		return checks.ValidationResult{
			OK:  true,
			Msg: "no duplicate header columns",
		}
	}

	return checks.ValidationResult{
		OK:  false,
		Msg: "duplicate header columns: " + strings.Join(dups, ", "),
	}
}

func readDuplicateHeader(
	ctx context.Context,
	data []byte,
) ([]string, checks.ValidationResult, bool) {
	r := checks.NewSemicolonCSVReader(data)

	for {
		if err := ctx.Err(); err != nil {
			return nil, cancelledValidation(err), false
		}

		rec, err := r.Read()
		if err != nil || rec == nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, cancelledValidation(ctxErr), false
			}

			return nil, checks.ValidationResult{
				OK:  true,
				Msg: "no header line found (nothing to check for duplicates)",
			}, false
		}

		if !isBlankHeaderRecord(rec) {
			return rec, checks.ValidationResult{}, true
		}
	}
}

func isBlankHeaderRecord(record []string) bool {
	for _, col := range record {
		if !checks.IsBlankUnicode([]byte(col)) {
			return false
		}
	}

	return true
}

type duplicateHeaderStat struct {
	count    int
	sample   string
	reported bool
}

func findDuplicateHeaderCells(ctx context.Context, header []string) ([]string, error) {
	seen := make(map[string]*duplicateHeaderStat, len(header))
	var duplicateOrder []string

	for _, col := range header {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		key := duplicateHeaderKey(col)
		sample := duplicateHeaderSample(col)

		stat, ok := seen[key]
		if !ok {
			seen[key] = &duplicateHeaderStat{
				count:  1,
				sample: sample,
			}
			continue
		}

		stat.count++
		if !stat.reported {
			duplicateOrder = append(duplicateOrder, key)
			stat.reported = true
		}
	}

	dups := make([]string, 0, len(duplicateOrder))
	for _, key := range duplicateOrder {
		stat := seen[key]
		dups = append(dups, stat.sample+"("+strconv.Itoa(stat.count)+")")
	}

	return dups, nil
}

func duplicateHeaderKey(col string) string {
	return strings.ToLower(strings.TrimSpace(col))
}

func duplicateHeaderSample(col string) string {
	sample := strings.TrimSpace(col)
	if sample == "" {
		return `"<empty>"`
	}

	return sample
}

func cancelledValidation(err error) checks.ValidationResult {
	return checks.ValidationResult{
		OK:  false,
		Msg: "validation cancelled",
		Err: err,
	}
}
