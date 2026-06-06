package repl

import (
	"bufio"
	"io"
	"os"

	"github.com/buildkite/conditional/internal/evaluator"
	"github.com/buildkite/conditional/internal/lexer"
	"github.com/buildkite/conditional/internal/object"
	"github.com/buildkite/conditional/internal/parser"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.Struct{
		"env": object.Function(envFunction),
	}

	for {
		io.WriteString(out, PROMPT)
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

func envFunction(args []object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "env expects exactly one argument"}
	}

	name, ok := args[0].(*object.String)
	if !ok {
		return &object.Error{Message: "env argument must be a string"}
	}

	value, ok := os.LookupEnv(name.Value)
	if !ok {
		return evaluator.NULL
	}

	return &object.String{Value: value}
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, "Parser errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
