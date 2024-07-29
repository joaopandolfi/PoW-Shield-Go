package router

import (
	"pow-shield-go/config"
	powServices "pow-shield-go/services/pow"
	"pow-shield-go/web/controllers/pow"
	"pow-shield-go/web/server"

	"github.com/unrolled/secure"
)

// Router public struct
type Router struct {
	s *server.Server
}

// New Router
func New(s *server.Server) Router {
	return Router{s: s}
}

// Setup router
func (r *Router) Setup() {
	r.secure()

	generator := powServices.NewGerator()
	verifier := powServices.NewVerifier()

	pow.New(generator, verifier).SetupRouter(r.createSubRouter("/pow"))
}

// CreateSubRouter with path
func (r *Router) createSubRouter(path string) *server.Server {
	return &server.Server{
		R:      r.s.R.PathPrefix(path).Subrouter(),
		Config: r.s.Config,
	}
}

func (r *Router) secure() {
	secureMiddleware := secure.New(config.Get().SecOptions)
	r.s.R.Use(secureMiddleware.Handler)
}
