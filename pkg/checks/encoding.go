package checks

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func StripUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, utf8BOM)
}

// isBlankUnicode reports whether the line consists only of Unicode whitespace
// plus additional zero-width/invisible code points that are commonly present
// in "blank-looking" lines (ZWSP, ZWNJ, ZWJ, WORD JOINER, BOM, etc.).
func IsBlankUnicode(b []byte) bool {
	// Extra invisibles not covered by unicode.IsSpace.
	switch {
	// Fast-path: empty slice
	case len(b) == 0:
		return true
	}
	extra := func(r rune) bool {
		switch r {
		case '\u200B', // ZERO WIDTH SPACE
			'\u200C', // ZERO WIDTH NON-JOINER
			'\u200D', // ZERO WIDTH JOINER
			'\u2060', // WORD JOINER
			'\ufeff', // BOM
			'\u180E': // MONGOLIAN VOWEL SEPARATOR (deprecated but still seen)
			return true
		}
		return false
	}

	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			// Treat undecodable byte as non-blank.
			return false
		}
		if !unicode.IsSpace(r) && !extra(r) {
			return false
		}
		i += size
	}
	return true
}

func SplitUTF8BOM(data []byte) ([]byte, []byte) {
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return data, nil
	}

	return data[3:], []byte{0xEF, 0xBB, 0xBF}
}
