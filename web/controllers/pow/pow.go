package pow

import (
	"net/http"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/server"
)

// --- PoW ---

type controller struct {
	s *server.Server
}

// New controller
func New() controllers.Controller {
	return &controller{
		s: nil,
	}
}

// challenge - get PoW challenge
func (c *controller) challenge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	handler.RespondJson(w, true, http.StatusOK)
}

// verify - verify PoW challenge
func (c *controller) verify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	handler.RespondJson(w, true, http.StatusOK)
}
