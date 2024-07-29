package handler

import (
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/models/domain"
)

const TOKEN_SESSION = "token"

func SetSession(w http.ResponseWriter, r *http.Request, session *domain.Session) error {
	s, _ := config.Get().Session.Store.Get(r, config.Get().Session.Name)

	s.Values[TOKEN_SESSION] = session
	return s.Save(r, w)
}

func GetSession(r *http.Request) *domain.Session {
	s, _ := config.Get().Session.Store.Get(r, config.Get().Session.Name)
	sess, ok := s.Values[TOKEN_SESSION]
	if !ok {
		return nil
	}

	decodedSess, ok := sess.(domain.Session)
	if !ok {
		return nil
	}

	return &decodedSess
}
