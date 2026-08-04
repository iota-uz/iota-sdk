package bootstrap

import (
	"log"
	"os"
	"runtime/debug"
)

func Main(run func() error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	if err := run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
