package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func Hash(buffer []byte, prefix string) ([]byte, error) {
	h := sha256.New()

	if prefix != "" {
		prefixBytes, err := hex.DecodeString(prefix)
		if err != nil {
			return nil, fmt.Errorf("decoding hex string: %w", err)
		}
		h.Write(prefixBytes)
	}

	h.Write(buffer)
	return h.Sum(nil), nil
}

func CheckComplexity(hash []byte, complexity int) (bool, error) {
	if complexity >= len(hash)*8 {
		return false, fmt.Errorf("complexity is too high")
	}

	off := 0
	i := 0
	for i <= complexity-8 {
		if hash[off] != 0 {
			return false, nil
		}
		i += 8
		off++
	}

	mask := 0xff << (8 + i - complexity)
	return (hash[off] & byte(mask)) == 0, nil
}
