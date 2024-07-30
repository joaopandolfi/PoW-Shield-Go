package pow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/models/domain"
)

type Generator interface {
	Problem(ctx context.Context, requester *domain.Session) (*domain.Challenge, error)
}

type generator struct {
	defaultProblemLength int
	cache                cache.Cache
}

func NewGerator() Generator {
	return &generator{
		defaultProblemLength: config.Get().Pow.DefaultPrefixSize,
		cache:                cache.Get(),
	}
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length/2) // Each hex character represents half a byte
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func (s *generator) Problem(ctx context.Context, requester *domain.Session) (*domain.Challenge, error) {

	challenge := domain.Challenge{
		Requester: requester.ID.String(),
	}

	previousChallenge, err := s.cache.Get(requester.ID.String())
	if err != nil {
		return nil, fmt.Errorf("getting previuos challenge: %w", err)
	}

	if previousChallenge != nil {
		state, _ := previousChallenge.(string)
		challenge.ParsePreviousState(state)
	}

	difficulty := s.defaultProblemLength + requester.Difficulty + challenge.Difficulty
	prefix, err := generateRandomString(difficulty)
	if err != nil {
		return nil, fmt.Errorf("generating prefix: %w", err)
	}

	challenge.Prefix = prefix
	challenge.Difficulty = difficulty

	err = s.cache.Put(challenge.Key(), domain.CHALLENGE_STATUS_TO_SOLVE, defaultCacheDuration)
	if err != nil {
		return nil, fmt.Errorf("error on saving problem on cache: %w", err)
	}

	return &challenge, nil
}
