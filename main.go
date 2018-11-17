package main

import (
	"fmt"
	"os"

	"github.com/buildkite/condition/repl"
)

func main() {
	fmt.Printf("Buildkite condition evaluator\n")
	repl.Start(os.Stdin, os.Stdout)
}
