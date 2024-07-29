package pow

import "pow-shield-go/web/server"

// SetupRouter -
func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	c.s.R.HandleFunc("/pow", c.challenge).Methods("GET", "HEAD")
	c.s.R.HandleFunc("/pow", c.verify).Methods("POST", "HEAD")
}
