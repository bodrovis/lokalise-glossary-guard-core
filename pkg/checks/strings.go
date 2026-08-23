package checks

import (
	"strings"
)

func DetectLineEnding(b []byte) string {
	crlf := 0
	lf := 0
	for i, ch := range b {
		if ch == '\n' {
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

func AnyNonEmpty(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}
