package empty_lines

import (
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type emptyLinesReport struct {
	total int
	first []int
}

func scanEmptyLines(
	ctx context.Context,
	data []byte,
) (emptyLinesReport, error) {
	scanner := checks.NewLineScanner(data, maxScannedLineSize)

	var report emptyLinesReport

	for lineNo := 1; scanner.Scan(); lineNo++ {
		if err := checkContextEveryLine(ctx, lineNo); err != nil {
			return emptyLinesReport{}, err
		}

		line := checks.TrimTrailingCR(scanner.Bytes())

		if checks.IsBlankUnicode(line) {
			report.add(lineNo)
		}
	}

	if err := scanner.Err(); err != nil {
		return emptyLinesReport{}, err
	}

	return report, nil
}
