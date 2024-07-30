package domain

import (
	"encoding/json"
	"fmt"
	"pow-shield-go/config"
	"pow-shield-go/services/utils"
	"strings"

	"github.com/google/uuid"
)

type Session struct {
	Authorized bool
	Difficulty int
	Prefix     string
	Buffer     string
	ID         uuid.UUID
}

func NewSession() *Session {
	return &Session{
		ID:         uuid.New(),
		Difficulty: 0,
	}
}

func (s *Session) Wrap() string {
	b, _ := json.Marshal(s)
	return string(utils.ToBase64(b))
}

func (s *Session) Unrap(data string) error {
	decoded, err := utils.FromBase64(data)
	if err != nil {
		return fmt.Errorf("reding from base64: %w", err)
	}
	return json.Unmarshal([]byte(decoded), s)
}

func (s *Session) ToCookie() *Cookie {
	config := config.Get().Session
	return &Cookie{
		Value:    s.Wrap(),
		MaxAge:   config.Options.MaxAge,
		HttpOnly: config.Options.HttpOnly,
		Secure:   config.Options.Secure,
		Path:     "/",
	}
}

func (s *Session) ValidSessionState(state string) bool {
	return !(strings.Contains(state, CHALLENGE_STATUS_ERROR_COUNT) || state == CHALLENGE_STATUS_TO_SOLVE)
}
