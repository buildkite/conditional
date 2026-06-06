package evaluator

import (
	"testing"

	"github.com/buildkite/conditional/internal/lexer"
	"github.com/buildkite/conditional/internal/object"
	"github.com/buildkite/conditional/internal/parser"
)

// An example of evaluating a single token to get a baseline of evaluator overhead
func BenchmarkSingleTokenEvaluation(bench *testing.B) {
	l := lexer.New(`true`)
	p := parser.New(l)
	expr := p.Parse()
	env := &object.Struct{}
	bench.ResetTimer()

	for i := 0; i < bench.N; i++ {
		_ = Eval(expr, env)
	}
}

func BenchmarkSimpleEvaluation(bench *testing.B) {
	l := lexer.New(`build.branch == "master" && build.pull_request.draft == true`)
	p := parser.New(l)
	expr := p.Parse()
	env := &object.Struct{
		`build.branch`:             &object.String{Value: "master"},
		`build.pull_request.draft`: &object.Boolean{Value: true},
	}
	bench.ResetTimer()

	for i := 0; i < bench.N; i++ {
		_ = Eval(expr, env)
	}
}
