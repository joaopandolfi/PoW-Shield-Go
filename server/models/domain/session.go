package domain

import (
	"encoding/json"
	"fmt"

	"pow-shield-go/config"
	"pow-shield-go/services/utils"
	"strings"
	"time"
)

type Session struct {
	Authorized  bool
	Difficulty  int
	Prefix      string
	Buffer      string
	Requests    int
	Challenges  int
	CreatedAt   time.Time
	LastRequest time.Time
	ID          string `json:"id"`
}

func NewSession(id string) *Session {
	return &Session{
		ID:          id,
		Difficulty:  0,
		Requests:    0,
		Challenges:  0,
		CreatedAt:   time.Now(),
		LastRequest: time.Now(),
	}
}

func (s *Session) Wrap() string {
	b, _ := json.Marshal(s)
	return string(utils.ToBase64(b))
}

func (s *Session) PublicWrap() string {
	// strip data who cannot be public
	s.Requests = 0
	s.Challenges = 0
	return s.Wrap()
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
	if strings.Contains(state, CHALLENGE_STATUS_ERROR_COUNT) || strings.Contains(state, CHALLENGE_STATUS_TO_SOLVE) {
		return false
	}

	return VerifyChallengeState(state, s.Buffer)
}

func (s *Session) ContabilizeNewRequest() {
	s.Requests += 1
	s.LastRequest = time.Now()
}

func (s *Session) RegisterNewChallenge(success bool, prefix, buffer string) {
	s.Authorized = success
	s.Prefix = prefix
	s.Buffer = buffer
	s.Challenges += 1
}
