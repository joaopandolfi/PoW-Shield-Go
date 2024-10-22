package pow

import (
	"context"
	"encoding/hex"
	"fmt"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/models/domain"
	"pow-shield-go/services/utils"
)

const minNonceSize = 8
const maxNonceSize = 32

type Verifier interface {
	Verify(ctx context.Context, session *domain.Session, nonce []byte, compelexity int, prefix string) (bool, error)
}

type verifier struct {
	validity   uint64
	punishment int
	cache      cache.Cache
}

func NewVerifier() Verifier {
	return &verifier{
		validity:   uint64(config.Get().Pow.NonceValidity),
		punishment: config.Get().Pow.Punishment,
		cache:      cache.Get(),
	}
}

// Verify nonce
func (s *verifier) Verify(ctx context.Context, session *domain.Session, nonce []byte, compelexity int, prefix string) (bool, error) {

	success := false
	defer func() {
		key := session.ID
		challenge := domain.NewChallenge()
		previousChallenge, _ := s.cache.Get(key)

		if previousChallenge != nil {
			previousState, ok := previousChallenge.(string)
			if ok {
				challenge.Status = previousState
			}
		}
		if !success {
			s.cache.Put(key, challenge.IncreaseDifficulty(challenge.Status, s.punishment), defaultCacheDuration)
			session.Difficulty = challenge.Difficulty
		} else {
			s.cache.Put(key, challenge.RegisterSuccess(hex.EncodeToString(nonce), s.punishment), defaultCacheDuration)
		}
	}()

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

	success = valid

	return valid, nil
}
