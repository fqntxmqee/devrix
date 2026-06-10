package main

import (
	"fmt"
	"os"

	"github.com/devrix/devrix/internal/cli/debug"
)

func main() {
	if err := debug.RunExport(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
