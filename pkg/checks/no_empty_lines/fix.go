package empty_lines

import (
	"bufio"
	"bytes"
	"context"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// fixRemoveEmptyLines drops blank lines (lines that are all whitespace).
// It preserves the predominant line ending style (LF or CRLF) in the output.
func fixRemoveEmptyLines(ctx context.Context, a checks.Artifact) (checks.FixResult, error) {
	if err := ctx.Err(); err != nil {
		return checks.FixResult{}, err
	}

	in := a.Data
	if len(in) == 0 {
		return checks.FixResult{
			Data:      in,
			Path:      "",
			DidChange: false,
			Note:      "empty file",
		}, nil
	}

	sep := detectLineEnding(in)

	sc := bufio.NewScanner(bytes.NewReader(in))
	var out bytes.Buffer
	wroteAny := false
	dropped := 0

	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return checks.FixResult{}, err
		}
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			dropped++
			continue
		}
		if wroteAny {
			out.WriteString(sep)
		}
		out.Write(line)
		wroteAny = true
	}

	if dropped == 0 {
		return checks.FixResult{
			Data:      a.Data,
			Path:      "",
			DidChange: false,
			Note:      "no empty lines to remove",
		}, nil
	}

	note := "removed empty lines"
	if dropped == 1 {
		note = "removed 1 empty line"
	}

	return checks.FixResult{
		Data:      out.Bytes(),
		Path:      "",
		DidChange: true,
		Note:      note,
	}, nil
}

func detectLineEnding(b []byte) string {
	crlf := 0
	lf := 0
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' {
			if i > 0 && b[i-1] == '\r' {
				crlf++
			} else {
				lf++
			}
		}
	}
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}
