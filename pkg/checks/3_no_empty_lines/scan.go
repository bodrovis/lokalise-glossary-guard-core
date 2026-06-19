package empty_lines

import (
	"bufio"
	"bytes"
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

type emptyLinesReport struct {
	total int
	first []int
}

func newLineScanner(data []byte) *bufio.Scanner {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64<<10), maxScannedLineSize)

	return scanner
}

func scanEmptyLines(ctx context.Context, data []byte) (emptyLinesReport, error) {
	scanner := newLineScanner(data)

	var report emptyLinesReport

	for lineNo := 1; scanner.Scan(); lineNo++ {
		if err := checkContextEveryLine(ctx, lineNo); err != nil {
			return emptyLinesReport{}, err
		}

		line := normalizeScannedLine(scanner.Bytes())
		if checks.IsBlankUnicode(line) {
			report.add(lineNo)
		}
	}

	if err := scanner.Err(); err != nil {
		return emptyLinesReport{}, err
	}

	return report, nil
}

func normalizeScannedLine(line []byte) []byte {
	if n := len(line); n > 0 && line[n-1] == '\r' {
		return line[:n-1]
	}

	return line
}
