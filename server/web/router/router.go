package router

import (
	"pow-shield-go/config"
	powServices "pow-shield-go/services/pow"
	"pow-shield-go/web/controllers/health"
	"pow-shield-go/web/controllers/pow"
	"pow-shield-go/web/controllers/proxy"
	"pow-shield-go/web/controllers/static"
	"pow-shield-go/web/middleware"
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
	middleware.Setup()
	r.s.R.Use(middleware.Identificator)

	static.New().SetupRouter(r.s)
	generator := powServices.NewGerator()
	verifier := powServices.NewVerifier()

	pow.New(generator, verifier).SetupRouter(r.createSubRouter("/pow"))
	health.New().SetupRouter(r.s)
	proxy.New().SetupRouter(r.s)
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
