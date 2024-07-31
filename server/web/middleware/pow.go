package middleware

import (
	"log"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
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
		if config.Get().Pow.UseCookie {
			cookie, err := handler.GetCookie(r)
			if err != nil {
				log.Println("[!][Middleware][proxy] error on getting cookie", err.Error())
				blockRequest(w)
				return
			}

			if cookie == nil {
				log.Println("[*][Middleware][proxy] session not authorized")
				blockRequest(w)
				return
			}
		}

		session := handler.GetSession(r)
		if !session.Authorized {
			log.Println("[*][Middleware][proxy] session not authorized")
			blockRequest(w)
			return
		}

		sessionStatus, _ := powCache.Get(session.ID.String())
		if sessionStatus == nil {
			log.Println("[*][Middleware][proxy] cached session not found")
			blockRequest(w)
			return
		}

		status, _ := sessionStatus.(string)

		if !session.ValidSessionState(status) {
			log.Println("[*][Middleware][proxy] invalid session status")
			blockRequest(w)
			return
		}

		next(w, r)
	}
}
