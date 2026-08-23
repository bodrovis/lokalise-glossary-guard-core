package valid_encoding

import (
	"context"
)

const ctxCheckEvery = 1 << 20

func padBytes(data []byte, size int) []byte {
	if rem := len(data) % size; rem == 0 {
		return data
	} else {
		padded := make([]byte, len(data)+(size-rem))
		copy(padded, data)
		return padded
	}
}

func checkContextEvery(ctx context.Context, i int) error {
	if i&(ctxCheckEvery-1) != 0 {
		return nil
	}

	return ctx.Err()
}
