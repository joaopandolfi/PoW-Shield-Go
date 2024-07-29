package health

import "pow-shield-go/web/server"

// SetupRouter -
func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	c.s.R.HandleFunc("/health", c.health).Methods("POST", "GET", "HEAD")
}
