package checks

import (
	"bytes"
	"context"
)

type PhysicalLineParts struct {
	Before []byte
	Line   []byte
	Rest   []byte
}

func FindFirstNonBlankPhysicalLine(
	ctx context.Context,
	data []byte,
) (PhysicalLineParts, bool, error) {
	pos := 0

	for pos <= len(data) {
		if err := ctx.Err(); err != nil {
			return PhysicalLineParts{}, false, err
		}

		line, rest, found := bytes.Cut(
			data[pos:],
			[]byte("\n"),
		)

		lineForCheck := line
		if len(lineForCheck) > 0 &&
			lineForCheck[len(lineForCheck)-1] == '\r' {
			lineForCheck = lineForCheck[:len(lineForCheck)-1]
		}

		if !IsBlankUnicode(lineForCheck) {
			lineEnd := len(data) - len(rest)
			if !found {
				lineEnd = len(data)
			}

			return PhysicalLineParts{
				Before: data[:pos],
				Line:   data[pos:lineEnd],
				Rest:   data[lineEnd:],
			}, true, nil
		}

		if !found {
			break
		}

		pos += len(line) + 1
	}

	return PhysicalLineParts{}, false, nil
}
