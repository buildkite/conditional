package lexer

import (
	"testing"

	"github.com/buildkite/condition/token"
)

// An example of lexing a single token to get a baseline of lexing performance
func BenchmarkSingleTokenLex(bench *testing.B) {
	for i := 0; i < bench.N; i++ {
		l := New(`true`)
		l.NextToken()
	}
}

func BenchmarkSimpleLex(bench *testing.B) {
	for i := 0; i < bench.N; i++ {
		l := New(`build.branch == master" && build.pull_request == true`)
		for {
			if l.NextToken().Type == token.EOF {
				break
			}
		}
	}
}
