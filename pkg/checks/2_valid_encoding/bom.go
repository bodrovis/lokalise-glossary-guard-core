package valid_encoding

import (
	"bytes"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type bomKind int

const (
	bomNone bomKind = iota
	bomUTF8
	bomUTF16LE
	bomUTF16BE
	bomUTF32LE
	bomUTF32BE
)

func sniffBOM(b []byte) bomKind {
	switch {
	case len(b) >= 3 && bytes.Equal(b[:3], utf8BOM):
		return bomUTF8
	case len(b) >= 4 && b[0] == 0x00 && b[1] == 0x00 && b[2] == 0xFE && b[3] == 0xFF:
		return bomUTF32BE
	case len(b) >= 4 && b[0] == 0xFF && b[1] == 0xFE && b[2] == 0x00 && b[3] == 0x00:
		return bomUTF32LE
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF:
		return bomUTF16BE
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE:
		return bomUTF16LE
	default:
		return bomNone
	}
}
