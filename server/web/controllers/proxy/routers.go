package proxy

import (
	"pow-shield-go/web/middleware"
	"pow-shield-go/web/server"
)

// SetupRouter -
func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	c.s.R.PathPrefix("/").HandlerFunc(middleware.Waf(middleware.PoW(c.proxy)))
}
