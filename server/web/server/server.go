package server

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"pow-shield-go/config"
	"pow-shield-go/internal/logging"

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
	log := logging.Get()
	if log != nil {
		log.Info("Server starting", "port", conf.Port)
	}
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
		log := logging.Get()
		if log != nil {
			log.Error("Fatal server error", "error", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "FATAL: Server error: %v\n", err)
		}
	}
}

// Shutdown server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
