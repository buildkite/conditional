package evaluator

import (
	"testing"

	"github.com/buildkite/conditional/lexer"
	"github.com/buildkite/conditional/object"
	"github.com/buildkite/conditional/parser"
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
	l := lexer.New(`build.branch == master" && build.pull_request == true`)
	p := parser.New(l)
	expr := p.Parse()
	env := &object.Struct{
		`build`: &object.Struct{
			`branch`:       &object.String{Value: "master"},
			`pull_request`: &object.Boolean{Value: true},
		},
	}
	bench.ResetTimer()

	for i := 0; i < bench.N; i++ {
		_ = Eval(expr, env)
	}
}
