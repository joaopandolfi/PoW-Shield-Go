package pow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"pow-shield-go/config"
)

type Generator interface {
	Problem(ctx context.Context) (string, error)
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

func (s *generator) Problem(ctx context.Context) (string, error) {
	return generateRandomString(s.defaultProblemLength)
}
