package lexer

import (
	"testing"

	"github.com/buildkite/conditional/internal/token"
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
	expectTokens(t, `null`, []tokenExpectation{
		{token.NULL, `null`},
	})
	expectTokens(t, `"true"`, []tokenExpectation{
		{token.STRING, `true`},
	})
}

func TestLexingUnterminatedStrings(t *testing.T) {
	expectTokens(t, `"from prison \`, []tokenExpectation{
		{token.ILLEGAL, `from prison \`},
	})
	expectTokens(t, `'mad lad opening single quotes`, []tokenExpectation{
		{token.ILLEGAL, `mad lad opening single quotes`},
	})
}

func TestLexingStringEscapes(t *testing.T) {
	expectTokens(t, `"line\nfeed"`, []tokenExpectation{
		{token.STRING, "line\nfeed"},
	})
	expectTokens(t, `"space\sescape"`, []tokenExpectation{
		{token.STRING, "space escape"},
	})
	expectTokens(t, `"hex\x41 octal\141"`, []tokenExpectation{
		{token.STRING, "hexA octala"},
	})
	expectTokens(t, `"byte\xff octal\377"`, []tokenExpectation{
		{token.STRING, "byte" + string([]byte{0xff}) + " octal" + string([]byte{0xff})},
	})
	expectTokens(t, `"unknown\qescape"`, []tokenExpectation{
		{token.STRING, "unknownqescape"},
	})
	expectTokens(t, `'listening to \'music\''`, []tokenExpectation{
		{token.STRING, "listening to 'music'"},
	})
	expectTokens(t, `'\n stays literal'`, []tokenExpectation{
		{token.STRING, `\n stays literal`},
	})
	expectTokens(t, `'\\ becomes slash'`, []tokenExpectation{
		{token.STRING, `\ becomes slash`},
	})
}

func TestLexingValueComparisons(t *testing.T) {
	expectTokens(t, `build.branch == "master"`, []tokenExpectation{
		{token.IDENT, "build.branch"},
		{token.EQ, "=="},
		{token.STRING, "master"},
	})
	expectTokens(t, `(build.tag != "v1.0.0")`, []tokenExpectation{
		{token.LPAREN, "("},
		{token.IDENT, "build.tag"},
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
	expectTokens(t, `build.env("FOO") == "BAR"`, []tokenExpectation{
		{token.IDENT, "build.env"},
		{token.LPAREN, "("},
		{token.STRING, "FOO"},
		{token.RPAREN, ")"},
		{token.EQ, "=="},
		{token.STRING, "BAR"},
	})
}

func TestLexingRegexps(t *testing.T) {
	expectTokens(t, `build.tag =~ /^v/`, []tokenExpectation{
		{token.IDENT, "build.tag"},
		{token.RE_EQ, "=~"},
		{token.REGEXP, "^v"},
	})
	expectTokens(t, `build.branch =~ /^features\//`, []tokenExpectation{
		{token.IDENT, "build.branch"},
		{token.RE_EQ, "=~"},
		{token.REGEXP, `^features\/`},
	})
	expectTokens(t, `build.branch =~ /\/release-123\$/`, []tokenExpectation{
		{token.IDENT, "build.branch"},
		{token.RE_EQ, "=~"},
		{token.REGEXP, `\/release-123\$`},
	})
	expectTokens(t, `build.message !~ /\[skip tests\]/i`, []tokenExpectation{
		{token.IDENT, "build.message"},
		{token.RE_NOT_EQ, "!~"},
		{token.REGEXP, `\[skip tests\]`},
	})

	l := New(`build.message !~ /\[skip tests\]/i`)
	for {
		tok := l.NextToken()
		if tok.Type == token.REGEXP {
			if tok.Flags != "i" {
				t.Fatalf("regexp flags wrong. expected=%q, got=%q", "i", tok.Flags)
			}
			return
		}
		if tok.Type == token.EOF {
			t.Fatal("regexp token not found")
		}
	}
}

func TestLexingUnterminatedRegexp(t *testing.T) {
	expectTokens(t, `build.branch =~ /release`, []tokenExpectation{
		{token.IDENT, "build.branch"},
		{token.RE_EQ, "=~"},
		{token.ILLEGAL, "release"},
	})
}

func TestLexingArrays(t *testing.T) {
	expectTokens(t, `build.creator.teams includes "deploy"`, []tokenExpectation{
		{token.IDENT, "build.creator.teams"},
		{token.INCLUDES, "includes"},
		{token.STRING, "deploy"},
	})
}

func TestLexingRejectsAtGreaterOperator(t *testing.T) {
	expectTokens(t, `["llamas", "alpacas"] @> "alpacas"`, []tokenExpectation{
		{token.LBRACKET, "["},
		{token.STRING, "llamas"},
		{token.COMMA, ","},
		{token.STRING, "alpacas"},
		{token.RBRACKET, "]"},
		{token.ILLEGAL, "@"},
		{token.ILLEGAL, ">"},
		{token.STRING, "alpacas"},
	})
}

func TestLexingTernaries(t *testing.T) {
	expectTokens(t, `true ? false : true`, []tokenExpectation{
		{token.TRUE, "true"},
		{token.QUESTION, "?"},
		{token.FALSE, "false"},
		{token.COLON, ":"},
		{token.TRUE, "true"},
	})
}

func TestLexingShellExpansions(t *testing.T) {
	expectTokens(t, `$branch == "main"`, []tokenExpectation{
		{token.SHELL, "$branch"},
		{token.EQ, "=="},
		{token.STRING, "main"},
	})
	expectTokens(t, `${branch:-main} == "main"`, []tokenExpectation{
		{token.SHELL, "${branch:-main}"},
		{token.EQ, "=="},
		{token.STRING, "main"},
	})
	expectTokens(t, `${branch:${empty:-1}:${two+2}} == "ai"`, []tokenExpectation{
		{token.SHELL, "${branch:${empty:-1}:${two+2}}"},
		{token.EQ, "=="},
		{token.STRING, "ai"},
	})
}

func TestLexingUnterminatedShellExpansion(t *testing.T) {
	expectTokens(t, `${branch:-main == "main"`, []tokenExpectation{
		{token.ILLEGAL, `${branch:-main == "main"`},
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
