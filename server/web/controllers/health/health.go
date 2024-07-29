package health

import (
	"net/http"

	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/server"
)

// --- Health ---

type controller struct {
	s *server.Server
}

// New Health controller
func New() controllers.Controller {
	return &controller{
		s: nil,
	}
}

// Health route
func (c *controller) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	handler.RespondJson(w, true, http.StatusOK)
}
