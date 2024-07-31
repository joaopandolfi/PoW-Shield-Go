package middleware

import (
	"fmt"
	"log"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/models/domain"
	"pow-shield-go/web/handler"
)

var powCache cache.Cache

func InitPow() {
	if powCache == nil {
		powCache = cache.Get()
	}
}

func PoW(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		success := false
		blockReason := ""
		defer func() {
			if !success {
				log.Println("[+][Middleware][PoW] Blocking access to", r.URL.String(), handler.IP(r), "-", blockReason)
			}
		}()

		if config.Get().Pow.UseCookie {
			cookie, err := handler.GetCookie(r)
			if err != nil {
				blockReason = fmt.Sprintf("error on getting cookie: %s", err.Error())
				cleanAll(w, r)
				blockRequest(w)
				return
			}

			if cookie == nil {
				blockReason = "session not authorized"
				cleanAll(w, r)
				blockRequest(w)
				return
			}
		}

		var session *domain.Session
		if config.Get().Pow.UseSession {
			session = handler.GetSession(r)
		}

		if config.Get().Pow.UseHeader && session == nil {
			wrappedSession := handler.PowHeader(r)
			s := domain.Session{}
			err := s.Unrap(wrappedSession)
			if err != nil {
				cleanAll(w, r)
				blockReason = "header session found"
				blockRequest(w)
				return
			}
			session = &s
		}

		if session == nil {
			blockReason = "session not found"
			cleanAll(w, r)
			blockRequest(w)
			return
		}

		sessionStatus, _ := powCache.Get(session.ID)
		if sessionStatus == nil {
			blockReason = "cached session not found"
			cleanAll(w, r)
			blockRequest(w)
			return
		}

		status, _ := sessionStatus.(string)

		if !session.ValidSessionState(status) {
			blockReason = "invalid session status"
			cleanAll(w, r)
			blockRequest(w)
			return
		}

		success = true
		next(w, r)
	}
}
