package main

import (
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/commands"
)

func main() {
	err := commands.Migrate(modules.BuiltInModules...)
	if err != nil {
		panic(err)
	}
}
