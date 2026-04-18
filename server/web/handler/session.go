package handler

import (
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/models/domain"

	"github.com/gorilla/sessions"
)

const TOKEN_SESSION = "token"
const TEMP_TOKEN_SESSION = "pow-temp"

func SetSession(w http.ResponseWriter, r *http.Request, session *domain.Session) error {
	s, _ := config.Get().Session.Store.Get(r, config.Get().Session.Name)

	s.Values[TOKEN_SESSION] = session.Wrap()
	return s.Save(r, w)
}

func GetSession(r *http.Request) *domain.Session {
	s, _ := config.Get().Session.Store.Get(r, config.Get().Session.Name)
	sess, ok := s.Values[TOKEN_SESSION]
	if !ok {
		return nil
	}

	wrapSess, ok := sess.(string)
	if !ok {
		return nil
	}

	result := domain.Session{}
	err := result.Unrap(wrapSess)
	if err != nil {
		return nil
	}

	return &result
}

func CleanSessions(w http.ResponseWriter, r *http.Request) {
	s, _ := config.Get().Session.Store.Get(r, config.Get().Session.Name)

	if s != nil {
		s.Options.MaxAge = -1
		s.Save(r, w)
	}
}

func SetTempSessionValue(w http.ResponseWriter, r *http.Request, key, value string, maxAge int) error {
	s, _ := config.Get().Session.Fstore.Get(r, TEMP_TOKEN_SESSION)
	s.Options = &sessions.Options{
		Path:     config.Get().Session.Options.Path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   config.Get().Session.Options.Secure,
	}
	s.Values[key] = value
	return s.Save(r, w)
}

func GetTempSessionValue(r *http.Request, key string) (string, bool) {
	s, _ := config.Get().Session.Fstore.Get(r, TEMP_TOKEN_SESSION)
	raw, ok := s.Values[key]
	if !ok {
		return "", false
	}

	val, ok := raw.(string)
	if !ok {
		return "", false
	}

	return val, true
}

func CleanTempSessions(w http.ResponseWriter, r *http.Request) {
	s, _ := config.Get().Session.Fstore.Get(r, TEMP_TOKEN_SESSION)
	if s != nil {
		s.Options.MaxAge = -1
		s.Save(r, w)
	}
}
