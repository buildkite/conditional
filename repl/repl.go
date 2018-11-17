package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/buildkite/condition/evaluator"
	"github.com/buildkite/condition/lexer"
	"github.com/buildkite/condition/object"
	"github.com/buildkite/condition/parser"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()

	envStruct := &object.Struct{
		Props: map[string]object.Object{},
	}

	// add system environment to an env struct
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		envStruct.Props[parts[0]] = &object.String{Value: parts[1]}
	}

	env.Set(`env`, envStruct)

	for {
		fmt.Printf(PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		if line == `quit` || line == `exit` {
			return
		}

		l := lexer.New(line)
		p := parser.New(l)

		expr := p.Parse()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			continue
		}

		io.WriteString(out, "Expression: "+expr.String())
		io.WriteString(out, "\n")

		evaluated := evaluator.Eval(expr, env)
		if evaluated != nil {
			io.WriteString(out, evaluated.String())
			io.WriteString(out, "\n")
		}

	}
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, "Parser errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
