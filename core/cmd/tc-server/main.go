package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ClausValcaTD/Total-connect-beta/core/internal/server"
	"github.com/ClausValcaTD/Total-connect-beta/core/internal/vault"
	"github.com/ClausValcaTD/Total-connect-beta/core/internal/sync"
	"github.com/ClausValcaTD/Total-connect-beta/bridge/internal/rclone"
)

func main() {
	log.Println("Starting Total Connect gRPC Server...")

	// Initialize components
	v := vault.New()
	b := rclone.New()
	s := sync.New()

	// Create server
	srv := server.NewServer(v, b, s)

	log.Println("Initialized Vault, rclone Bridge, and Sync Engine")

	// Channel to catch server errors
	errChan := make(chan error, 1)

	go func() {
		log.Printf("gRPC server listening on %s\n", server.DefaultAddr)
		if err := srv.Start(); err != nil {
			errChan <- err
		}
	}()

	// Signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-quit:
		log.Printf("Received signal %v. Shutting down gracefully...\n", sig)
		srv.Stop()
		log.Println("Server stopped.")
	}
}
