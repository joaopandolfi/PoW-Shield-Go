package metrics

import (
	"net/http"
	"pow-shield-go/config"
	imetrics "pow-shield-go/internal/metrics"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/server"
)

type controller struct {
	s *server.Server
}

func New() controllers.Controller {
	return &controller{}
}

func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	if !config.Get().Metrics.Active {
		return
	}

	c.s.R.HandleFunc(config.Get().Metrics.Path, c.prometheus).Methods("GET", "HEAD")
}

func (c *controller) prometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(imetrics.Prometheus()))
}
