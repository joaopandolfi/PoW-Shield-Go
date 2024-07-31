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
	"strings"
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
		log.Println("[!][ERROR][proxy] reading body", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	for v, k := range config.Get().ProtectedServer.DefaultHeaders {
		w.Header().Add(v, k)
	}

	redirectHost := fmt.Sprintf("%s%s", config.Get().ProtectedServer.Host, r.URL.Path)

	result, reqCode, reqHeader, err := request.RequestWithHeader(
		r.Method, redirectHost,
		handler.Headers(r),
		body,
	)
	if err != nil {
		log.Println("[!][ERROR][proxy] proxing request: ", err.Error())
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}

	for v, k := range reqHeader {
		w.Header().Add(v, strings.Join(k, ","))
	}

	w.WriteHeader(reqCode)
	w.Write(result)
}
