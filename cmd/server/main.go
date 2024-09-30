package main

import (
	"github.com/iota-agency/iota-erp/internal/server"
	_ "github.com/lib/pq"
	"log"
)

func main() {
	srv, err := server.DefaultServer()
	if err != nil {
		log.Fatalf("failed to initialized the server: %v", err)
	}
	if err := srv.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
