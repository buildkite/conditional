package main

import (
	"fmt"
	"os"

	"github.com/buildkite/conditional/internal/repl"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "conformance" {
		if err := runConformance(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("Buildkite condition evaluator")
	repl.Start(os.Stdin, os.Stdout)
}
