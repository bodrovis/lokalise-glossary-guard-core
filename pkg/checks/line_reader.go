package checks

import (
	"bufio"
	"bytes"
)

func NewLineScanner(data []byte, maxLineSize int) *bufio.Scanner {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64<<10), maxLineSize)

	return scanner
}

func TrimTrailingCR(line []byte) []byte {
	if n := len(line); n > 0 && line[n-1] == '\r' {
		return line[:n-1]
	}

	return line
}
