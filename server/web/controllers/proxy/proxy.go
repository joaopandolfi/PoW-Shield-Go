package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/internal/request"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/server"
)

type controller struct {
	s     *server.Server
	cache cache.Cache
}

// New controller
func New() controllers.Controller {
	return &controller{
		s:     nil,
		cache: cache.Get(),
	}
}

func (s *controller) proxy(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("[ERROR][proxy] reading body", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	cookie, err := handler.GetCookie(r)
	if err != nil {
		log.Println("[ERROR][proxy] error on getting cookie", err.Error())
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	if cookie == nil {
		log.Println("[ERROR][proxy] invalid cookie")
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	session := handler.GetSession(r)
	if !session.Authorized {
		log.Println("[ERROR][proxy] session not authorized")
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	sessionStatus, _ := s.cache.Get(session.ID.String())
	if sessionStatus == nil {
		log.Println("[ERROR][proxy] cached session not found")
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	status, _ := sessionStatus.(string)

	if !session.ValidSessionState(status) {
		log.Println("[ERROR][proxy] invalid session status", status)
		handler.RespondDefaultError(w, http.StatusForbidden)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	redirectHost := fmt.Sprintf("%s%s", config.Get().ProtectedServer.Host, r.URL.Path)

	result, reqCode, err := request.RequestWithHeader(
		r.Method, redirectHost,
		handler.Headers(r),
		body,
	)
	if err != nil {
		log.Println("[ERROR][proxy] procying request", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	w.WriteHeader(reqCode)
	w.Write(result)
}
