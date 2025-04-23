package main

import (
	"runtime/debug"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/commands"
)

func panicWithStack(err error) {
	errorWithStack := string(debug.Stack()) + "\n\nError: " + err.Error()
	panic(errorWithStack)
}

func main() {
	// TODO: Add more commands here like so:
	// go run cmd/command/main.go check_tr_keys
	// go run cmd/command/main.go some_other_command

	if err := commands.CheckTrKeys(modules.BuiltInModules...); err != nil {
		panicWithStack(err)
	}
}
