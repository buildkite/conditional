package ast

import (
	"testing"

	"github.com/buildkite/evaluate/token"
)

func TestString(t *testing.T) {
	expr := &InfixExpression{
		Token: token.Token{Type: token.EQ, Literal: "=="},
		Left: &IntegerLiteral{
			Token: token.Token{Type: token.INT, Literal: "1"},
			Value: 1,
		},
		Operator: "==",
		Right: &IntegerLiteral{
			Token: token.Token{Type: token.INT, Literal: "1"},
			Value: 1,
		},
	}

	if expr.String() != "(1 == 1)" {
		t.Errorf("expr.String() wrong. got=%q", expr.String())
	}
}
