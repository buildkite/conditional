package lexer

import (
	"testing"

	"github.com/buildkite/condition/token"
)

type tokenExpectation struct {
	expectedType    token.TokenType
	expectedLiteral string
}

func TestLexingIndividualTerms(t *testing.T) {
	expectTokens(t, `true`, []tokenExpectation{
		{token.TRUE, `true`},
	})
	expectTokens(t, `false`, []tokenExpectation{
		{token.FALSE, `false`},
	})
	expectTokens(t, `"true"`, []tokenExpectation{
		{token.STRING, `true`},
	})
}

func TestLexingValueComparisons(t *testing.T) {
	expectTokens(t, `build.branch == "master"`, []tokenExpectation{
		{token.IDENT, "build"},
		{token.DOT, "."},
		{token.IDENT, "branch"},
		{token.EQ, "=="},
		{token.STRING, "master"},
	})
	expectTokens(t, `(build.tag != "v1.0.0")`, []tokenExpectation{
		{token.LPAREN, "("},
		{token.IDENT, "build"},
		{token.DOT, "."},
		{token.IDENT, "tag"},
		{token.NOT_EQ, "!="},
		{token.STRING, "v1.0.0"},
		{token.RPAREN, ")"},
	})
	expectTokens(t, `"blah" == 'blah'`, []tokenExpectation{
		{token.STRING, "blah"},
		{token.EQ, "=="},
		{token.STRING, "blah"},
	})
}

func TestLexingFunctionCalls(t *testing.T) {
	expectTokens(t, `env('FOO') == "BAR"`, []tokenExpectation{
		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.STRING, "FOO"},
		{token.RPAREN, ")"},
		{token.EQ, "=="},
		{token.STRING, "BAR"},
	})
	expectTokens(t, `env(env('BAR')) == "FOO"`, []tokenExpectation{
		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.STRING, "BAR"},
		{token.RPAREN, ")"},
		{token.RPAREN, ")"},
		{token.EQ, "=="},
		{token.STRING, "FOO"},
	})
}

func TestLexingRegexps(t *testing.T) {
	expectTokens(t, `build.tag =~ /^v/`, []tokenExpectation{
		{token.IDENT, "build"},
		{token.DOT, "."},
		{token.IDENT, "tag"},
		{token.RE_EQ, "=~"},
		{token.REGEXP, "^v"},
	})
}

func TestLexingArrays(t *testing.T) {
	expectTokens(t, `["llamas", "alpacas"] @> "alpacas"`, []tokenExpectation{
		{token.LBRACKET, "["},
		{token.STRING, "llamas"},
		{token.COMMA, ","},
		{token.STRING, "alpacas"},
		{token.RBRACKET, "]"},
		{token.CONTAINS, "@>"},
		{token.STRING, "alpacas"},
	})
}

func expectTokens(t *testing.T, input string, expect []tokenExpectation) {
	t.Helper()
	l := New(input)

	for i, tt := range expect {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("#%d - tokentype wrong. expected=%q, got=%q (%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("#%d - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}

	if tok := l.NextToken(); tok.Type != token.EOF {
		t.Fatalf("unexpected extra token, expected=EOF, got=%q (%q)",
			tok.Type, tok.Literal)
	}
}
