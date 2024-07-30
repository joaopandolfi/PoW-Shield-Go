package pow

import (
	"context"
	"fmt"
	"pow-shield-go/config"
	"pow-shield-go/services/utils"
)

const minNonceSize = 8
const maxNonceSize = 32

type Verifier interface {
	Verify(ctx context.Context, nonce []byte, compelexity int, prefix string) (bool, error)
}

type verifier struct {
	validity uint64
}

func NewVerifier() Verifier {
	return &verifier{
		validity: uint64(config.Get().Pow.NonceValidity),
	}
}

// Verify nonce
func (s *verifier) Verify(ctx context.Context, nonce []byte, compelexity int, prefix string) (bool, error) {
	nonceSize := len(nonce)
	if nonceSize < minNonceSize || nonceSize > maxNonceSize {
		return false, nil
	}

	timestamp := utils.ReadTimestamp(nonce, 0)
	now := utils.Now()
	diff := now - timestamp
	if diff > s.validity {
		return false, fmt.Errorf("expired tollerance for challenge")
	}

	hash, err := utils.Hash(nonce, prefix)
	if err != nil {
		return false, fmt.Errorf("hashing nonce: %w", err)
	}

	valid, err := utils.CheckComplexity(hash, compelexity)
	if err != nil {
		return false, fmt.Errorf("checking complexity: %w", err)
	}

	return valid, nil
}
