package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"pow-shield-go/config"
	"pow-shield-go/internal/cache"
	"pow-shield-go/web/router"
	"pow-shield-go/web/server"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

func configInit(ctx context.Context) {
	config.Load()

	tickMemoryGarbageCollector := time.Second * 45
	cache.Initialize(ctx, tickMemoryGarbageCollector)

}

func gracefullShutdown() {
	fmt.Println("<====================================Shutdown==================================>")
	c := cache.Get()
	if c != nil {
		c.GracefullShutdown()
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
	welcome()
	//Init
	ctx := context.Background()
	configInit(ctx)

	// Initialize Mux Router
	r := mux.NewRouter()
	r.Use(mux.CORSMethodMiddleware(r))

	srv := server.New(r, config.Get())
	nr := router.New(srv)
	nr.Setup()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go srv.Start()

	<-done
	log.Println("[SERVER] Gracefully shutdown start")
	gracefullShutdown()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed %s", err.Error())
	}

	cancel()
	log.Println("[SERVER] Gracefully shutdown finish")
}
