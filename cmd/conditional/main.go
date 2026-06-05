package main

import (
	"fmt"
	"os"

	"github.com/buildkite/conditional/repl"
)

func main() {
	fmt.Println("Buildkite condition evaluator")
	repl.Start(os.Stdin, os.Stdout)
}
