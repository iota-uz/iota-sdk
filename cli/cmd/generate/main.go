package main

import (
	"github.com/iota-agency/iota-erp/cli/internal"
)

func main() {
	err := internal.New().Run()
	if err != nil {
		panic(err)
	}
}
