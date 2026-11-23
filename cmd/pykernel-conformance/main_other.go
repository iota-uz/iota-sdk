//go:build !unix

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "pykernel-conformance: the pykernel sandbox requires Linux")
	os.Exit(2)
}
