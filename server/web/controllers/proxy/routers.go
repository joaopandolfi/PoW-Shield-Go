package proxy

import (
	"pow-shield-go/web/server"
)

// SetupRouter -
func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	c.s.R.PathPrefix("/").HandlerFunc(c.proxy)
}
