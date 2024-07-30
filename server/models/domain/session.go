package domain

import "encoding/json"

type Session struct {
	Authorized bool
	Difficulty int
	Prefix     string
	Buffer     string
}

func (s *Session) Wrap() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (s *Session) Unrap(data string) error {
	return json.Unmarshal([]byte(data), s)
}
