package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"plugtalk/internal/server"
)

func main() {
	var (
		host        string
		port        int
		versionFlag bool
	)

	flag.StringVar(&host, "host", "127.0.0.1", "Host for HTTP server")
	flag.IntVar(&port, "port", 8080, "Port number for HTTP server")
	flag.BoolVar(&versionFlag, "version", false, "Display version information")
	flag.Parse()

	if versionFlag {
		fmt.Println("version 1.0")
		return
	}

	// Create server with configured host and port
	_, srv := server.NewServer(host, port)

	// Setup a channel to listen for interrupt or terminal signals
	// to gracefully shutdown the server
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopChan // wait for terminal signal
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			log.Fatalf("Server shutdown error: %s", err)
		}
	}()

	// Start the server
	log.Printf("Starting server on %s:%d", host, port)
	err := srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Server failed to start: %s", err)
	}
}
