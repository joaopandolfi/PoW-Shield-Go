package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	lp "pow-shield-go/internal/logging"
	"pow-shield-go/web/router"
	"pow-shield-go/web/server"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

func configInit(ctx context.Context) error {
	err := config.Load()
	if err != nil {
		return fmt.Errorf("loading envs: %w", err)
	}

	logging := config.Get().Logging
	var lvl lp.Level
	switch logging.Level {
	case "DEBUG":
		lvl = lp.LevelDebug
	case "WARN":
		lvl = lp.LevelWarn
	case "ERROR":
		lvl = lp.LevelError
	default:
		lvl = lp.LevelInfo
	}

	lp.Init(lp.Options{
		Level:       lvl,
		FilePath:    logging.FilePath,
		Stacktrace:  logging.Stacktrace,
		Component:   "PoWShield",
		Environment: logging.Environment,
	})
	log := lp.Get()
	if log != nil {
		log.Info("Logging initialized", "level", logging.Level, "env", logging.Environment)
	}

	tickMemoryGarbageCollector := time.Second * 45
	cache.Initialize(ctx, tickMemoryGarbageCollector)
	return nil
}

func gracefulShutdown() {
	fmt.Println("<====================================Shutdown==================================>")
	log := lp.Get()
	if log != nil {
		log.Info("Graceful shutdown initiated")
	}
	c := cache.Get()
	if c != nil {
		c.GracefulShutdown()
	}
}

func welcome() {
	//https://patorjk.com/software/taag/#p=display&f=Doom&t=samples
	fmt.Println(`
	______     _    _       _     _      _     _   _____       
	| ___ \   | |  | |     | |   (_)    | |   | | |  __ \      
	| |_/ /__ | |  | |  ___| |__  _  ___| | __| | | |  \/ ___  
	|  __/ _ \| |/\| | / __| '_ \| |/ _ \ |/ _  | | | __ / _ \ 
	| | | (_) \  /\  / \__ \ | | | |  __/ | (_| | | |_\ \ (_) |
	\_|  \___/ \/  \/  |___/_| |_|_|\___|_|\__,_|  \____/\___/ 
															   			  
O=======================================(The winter is comming)====>
	`)
}

func main() {
	ctx := context.Background()
	err := configInit(ctx)
	if err != nil {
		log := lp.Get()
		if log != nil {
			log.Error("Failed to initialize", "error", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		}
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.Use(mux.CORSMethodMiddleware(r))

	srv := server.New(r, config.Get())
	nr := router.New(srv)
	nr.Setup()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go srv.Start()

	<-done
	gracefulShutdown()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := srv.Shutdown(ctx); err != nil {
		log := lp.Get()
		if log != nil {
			log.Error("Server shutdown failed", "error", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: Server shutdown failed: %v\n", err)
		}
		os.Exit(1)
	}
	cancel()

	log := lp.Get()
	if log != nil {
		log.Info("Server shutdown complete")
	}
}
