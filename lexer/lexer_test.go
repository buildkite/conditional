package lexer

import (
	"testing"

	"github.com/buildkite/evaluate/token"
)

func TestNextToken(t *testing.T) {
	input := `# individual terms
	true
	false

	# compare values
	1 == 1
	true != "false"
	"blah" == 'blah' # trailing comment

	# compare function calls
	env(FOO) == env(BAR)

	# compare function calls to values
	env(FOO) == "llamas"

	# nested function calls
	env(env(FOO))

	# regular expression matches
	"v1.0.0" =~ /^v/

	# parenthesis
	((env(TAG) =~ /^v/) && (env(BRANCH) == master)) || true
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.TRUE, "true"},
		{token.FALSE, "false"},

		{token.INT, "1"},
		{token.EQ, "=="},
		{token.INT, "1"},
		{token.TRUE, "true"},
		{token.NOT_EQ, "!="},
		{token.STRING, "false"},
		{token.STRING, "blah"},
		{token.EQ, "=="},
		{token.STRING, "blah"},

		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.IDENT, "FOO"},
		{token.RPAREN, ")"},
		{token.EQ, "=="},
		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.IDENT, "BAR"},
		{token.RPAREN, ")"},

		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.IDENT, "FOO"},
		{token.RPAREN, ")"},
		{token.EQ, "=="},
		{token.STRING, "llamas"},

		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.IDENT, "FOO"},
		{token.RPAREN, ")"},
		{token.RPAREN, ")"},

		{token.STRING, "v1.0.0"},
		{token.RE_EQ, "=~"},
		{token.REGEX, "/^v/"},

		{token.LPAREN, "("},
		{token.LPAREN, "("},
		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.IDENT, "TAG"},
		{token.RPAREN, ")"},
		{token.RE_EQ, "=~"},
		{token.REGEX, "/^v/"},
		{token.RPAREN, ")"},
		{token.AND, "&&"},
		{token.LPAREN, "("},
		{token.IDENT, "env"},
		{token.LPAREN, "("},
		{token.IDENT, "BRANCH"},
		{token.RPAREN, ")"},
		{token.EQ, "=="},
		{token.IDENT, "master"},
		{token.RPAREN, ")"},
		{token.RPAREN, ")"},
		{token.OR, "||"},
		{token.TRUE, "true"},

		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q (%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
