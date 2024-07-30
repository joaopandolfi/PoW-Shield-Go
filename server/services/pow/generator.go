package pow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"pow-shield-go/config"
	"pow-shield-go/models/domain"
)

type Generator interface {
	Problem(ctx context.Context) (*domain.Challenge, error)
}

type generator struct {
	defaultProblemLength int
}

func NewGerator() Generator {
	return &generator{
		defaultProblemLength: config.Get().Pow.DefaultPrefixSize,
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

func (s *generator) Problem(ctx context.Context) (*domain.Challenge, error) {
	prefix, err := generateRandomString(s.defaultProblemLength)
	if err != nil {
		return nil, fmt.Errorf("generating prefix: %w", err)
	}

	return &domain.Challenge{
		Prefix:     prefix,
		Difficulty: s.defaultProblemLength,
	}, nil
}
