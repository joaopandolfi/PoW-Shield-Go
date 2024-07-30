package static

import (
	"net/http"
	"pow-shield-go/config"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/server"
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

// SetupRouter -
func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	cfg := config.Get().Static
	if cfg.ServeStatic {
		c.s.R.Path("/favicon.ico").Handler(http.FileServer(http.Dir(cfg.StaticFolder)))
		c.s.R.PathPrefix(cfg.StaticPath).Handler(http.StripPrefix(cfg.StaticPath, http.FileServer(http.Dir(cfg.StaticFolder))))
	}
}
