package pow

import "pow-shield-go/web/server"

// SetupRouter -
func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	c.s.R.HandleFunc("/", c.challenge).Methods("GET", "HEAD")
	c.s.R.HandleFunc("/", c.verify).Methods("POST", "HEAD")
}
