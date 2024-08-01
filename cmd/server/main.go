package main

import (
	"github.com/iota-agency/iota-erp/internal/server"
	_ "github.com/lib/pq"
	"log"
)

func main() {
	srv := server.New()
	if err := srv.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
