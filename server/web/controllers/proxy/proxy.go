package proxy

import (
	"fmt"
	"io"
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/internal/logging"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/internal/request"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/server"
	"strings"
)

type controller struct {
	s *server.Server
}

// New controller
func New() controllers.Controller {
	return &controller{
		s: nil,
	}
}

func (s *controller) proxy(w http.ResponseWriter, r *http.Request) {
	metrics.IncRequest()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log := logging.Get()
		if log != nil {
			log.Error("Error reading request body", "error", err.Error())
		}
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
		log := logging.Get()
		if log != nil {
			log.Error("Proxy request failed", "url", redirectHost, "method", r.Method, "error", err.Error())
		}
		handler.RespondDefaultError(w, http.StatusBadRequest)
		return
	}
	metrics.IncProxied()

	for v, k := range reqHeader {
		w.Header().Add(v, strings.Join(k, ","))
	}

	w.WriteHeader(reqCode)
	w.Write(result)
}
