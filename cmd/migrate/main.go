package main

import (
	"github.com/go-faster/errors"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/commands"
	"log"
)

func main() {
	err := commands.Migrate()
	if errors.Is(err, application.ErrNoMigrationsFound) {
		log.Println("No migrations found")
		return
	}
	if err != nil {
		panic(err)
	}
}
