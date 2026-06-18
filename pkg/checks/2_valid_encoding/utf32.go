package valid_encoding

import (
	"context"
	"unicode/utf8"
)

func readUTF32(data []byte, be bool) uint32 {
	if be {
		return uint32(data[0])<<24 |
			uint32(data[1])<<16 |
			uint32(data[2])<<8 |
			uint32(data[3])
	}

	return uint32(data[3])<<24 |
		uint32(data[2])<<16 |
		uint32(data[1])<<8 |
		uint32(data[0])
}

func decodeUTF32(ctx context.Context, data []byte, be, skipBOM bool) ([]byte, error) {
	data = padBytes(data, 4)

	if skipBOM && len(data) >= 4 && readUTF32(data[:4], be) == 0x0000FEFF {
		data = data[4:]
	}

	out := make([]byte, 0, len(data)/2)

	for i := 0; i+3 < len(data); i += 4 {
		if err := checkContextEvery(ctx, i); err != nil {
			return nil, err
		}

		r := rune(readUTF32(data[i:i+4], be))
		if r == 0x0000FEFF {
			continue
		}
		if !utf8.ValidRune(r) {
			r = utf8.RuneError
		}

		out = utf8.AppendRune(out, r)
	}

	return out, nil
}
