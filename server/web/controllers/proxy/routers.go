package proxy

import (
	"pow-shield-go/config"
	"pow-shield-go/web/middleware"
	"pow-shield-go/web/server"
)

// SetupRouter -
func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	handler := c.proxy

	if config.Get().Pow.Active {
		handler = middleware.PoW(handler)
	}

	if config.Get().Waf.Active {
		handler = middleware.Waf(handler)
	}

	c.s.R.PathPrefix("/").HandlerFunc(handler)
}
