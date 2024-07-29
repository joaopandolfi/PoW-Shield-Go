package server

import (
	"context"
	"log"
	"net/http"

	"pow-shield-go/config"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Server web
type Server struct {
	R      *mux.Router
	Config config.Config
	srv    *http.Server
}

// New server
func New(r *mux.Router, conf config.Config) *Server {
	// Bind to a port and pass our router in
	log.Printf("Server listenning on %s", conf.Port)
	srv := &http.Server{
		Handler:      handlers.CompressHandler(r),
		Addr:         conf.Port,
		WriteTimeout: conf.WriteTimeout,
		ReadTimeout:  conf.ReadTimeout,
	}

	return &Server{
		R:      r,
		Config: conf,
		srv:    srv,
	}
}

// Start Web server
func (s *Server) Start() {
	cfg := config.Get()

	var err error
	if !config.Get().UseTLS {
		err = s.srv.ListenAndServe()
	} else {
		err = s.srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
	}
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Fatal server error %s", err.Error())
	}
}

// Shutdown server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
