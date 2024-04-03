package main

import (
	"fmt"
	"github.com/apollos-studio/sso/pkg/utils"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: makemigration <name>")
		os.Exit(1)
	}

	if err := utils.MkDirIfNone("migrations"); err != nil {
		panic(err)
	}
	name := strings.ReplaceAll(os.Args[1], " ", "_")

	t := time.Now().Format("20060102150405")
	file, err := os.Create(fmt.Sprintf("migrations/%s-%s.sql", t, name))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if _, err := file.WriteString("-- +migrate Up\n\n-- +migrate Down\n\n"); err != nil {
		panic(err)
	}
}
