package static

import (
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
}
