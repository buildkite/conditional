package parser

import (
	"fmt"
	"testing"

	"github.com/buildkite/conditional/ast"
	"github.com/buildkite/conditional/lexer"
)

func TestIntegerLiteralExpression(t *testing.T) {
	input := "5"

	l := lexer.New(input)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	literal, ok := expr.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("exp not *ast.IntegerLiteral. got=%T", expr)
	}
	if literal.Value != 5 {
		t.Errorf("literal.Value not %d. got=%d", 5, literal.Value)
	}
	if literal.TokenLiteral() != "5" {
		t.Errorf("literal.TokenLiteral not %s. got=%s", "5",
			literal.TokenLiteral())
	}
}

func TestStringLiteralExpression(t *testing.T) {
	input := `"llamas"`

	l := lexer.New(input)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	literal, ok := expr.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("exp not *ast.StringLiteral. got=%T", expr)
	}
	if literal.Value != "llamas" {
		t.Errorf("literal.Value not %q. got=%q", "llamas", literal.Value)
	}
	if literal.TokenLiteral() != "llamas" {
		t.Errorf("literal.TokenLiteral not %s. got=%s", "llamas",
			literal.TokenLiteral())
	}
}

func TestRegexpExpression(t *testing.T) {
	input := `/^llamas?/`

	l := lexer.New(input)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	literal, ok := expr.(*ast.Regexp)
	if !ok {
		t.Fatalf("exp not *ast.Regexp. got=%T", expr)
	}
	if literal.Regexp.String() != `^llamas?` {
		t.Errorf("regexp.String() not %q. got=%q", `^llamas?`, literal.Regexp.String())
	}
	if literal.TokenLiteral() != `^llamas?` {
		t.Errorf("regexp.TokenLiteral not %s. got=%s", `^llamas?`,
			literal.TokenLiteral())
	}
	if literal.Regexp.MatchTimeout != regexpMatchTimeout {
		t.Errorf("regexp.MatchTimeout not %v. got=%v", regexpMatchTimeout, literal.Regexp.MatchTimeout)
	}
}

func TestRegexpFlagsExpression(t *testing.T) {
	input := `/\[skip tests\]/i`

	l := lexer.New(input)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	literal, ok := expr.(*ast.Regexp)
	if !ok {
		t.Fatalf("exp not *ast.Regexp. got=%T", expr)
	}
	if literal.Flags != "i" {
		t.Errorf("regexp.Flags not %q. got=%q", "i", literal.Flags)
	}
}

func TestRegexpUnsupportedFlags(t *testing.T) {
	l := lexer.New(`/skip/x`)
	p := New(l)
	p.Parse()

	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors")
	}
}

func TestParsingPrefixExpressions(t *testing.T) {
	prefixTests := []struct {
		input    string
		operator string
		value    interface{}
	}{
		{"!5", "!", 5},
		{"!foobar", "!", "foobar"},
		{"!true", "!", true},
		{"!false", "!", false},
	}

	for _, tt := range prefixTests {
		l := lexer.New(tt.input)
		p := New(l)
		expr := p.Parse()
		checkParserErrors(t, p)

		prefixExpr, ok := expr.(*ast.PrefixExpression)
		if !ok {
			t.Fatalf("stmt is not ast.PrefixExpression. got=%T", expr)
		}
		if prefixExpr.Operator != tt.operator {
			t.Fatalf("exp.Operator is not '%s'. got=%s",
				tt.operator, prefixExpr.Operator)
		}
		if !testLiteralExpression(t, prefixExpr.Right, tt.value) {
			return
		}
	}
}

func TestParsingInfixExpressions(t *testing.T) {
	infixTests := []struct {
		input      string
		leftValue  interface{}
		operator   string
		rightValue interface{}
	}{
		{"5 == 5", 5, "==", 5},
		{"5 != 5", 5, "!=", 5},
		{`"a" == "a"`, "a", "==", "a"},
		{`"a" != "b"`, "a", "!=", "b"},
		{"foobar != barfoo", "foobar", "!=", "barfoo"},
		{"true == true", true, "==", true},
		{"true != false", true, "!=", false},
		{"false == false", false, "==", false},
	}

	for _, tt := range infixTests {
		l := lexer.New(tt.input)
		p := New(l)
		expr := p.Parse()
		checkParserErrors(t, p)

		if !testInfixExpression(t, expr, tt.leftValue,
			tt.operator, tt.rightValue) {
			return
		}
	}
}

func TestDottedIdentifierExpression(t *testing.T) {
	tests := []string{
		"foo.bar",
		"foo.bar.baz",
		"build.pull_request.repository.fork",
	}

	for _, input := range tests {
		l := lexer.New(input)
		p := New(l)
		expr := p.Parse()
		checkParserErrors(t, p)

		ident, ok := expr.(*ast.Identifier)
		if !ok {
			t.Fatalf("expr is not ast.Identifier. got=%T", expr)
		}
		if ident.Value != input {
			t.Fatalf("identifier value = %q, want %q", ident.Value, input)
		}
	}
}

func TestParserRejectsMalformedDottedIdentifiers(t *testing.T) {
	tests := []string{
		"foo.",
		"foo..bar",
	}

	for _, input := range tests {
		l := lexer.New(input)
		p := New(l)
		p.Parse()

		if len(p.Errors()) == 0 {
			t.Fatalf("expected parser errors for %q", input)
		}
	}
}

func TestParserRejectsUnterminatedStrings(t *testing.T) {
	tests := []string{
		`"from prison \`,
		`'mad lad opening single quotes`,
	}

	for _, input := range tests {
		l := lexer.New(input)
		p := New(l)
		p.Parse()

		if len(p.Errors()) == 0 {
			t.Fatalf("expected parser errors for %q", input)
		}
	}
}

func TestShellExpansionExpression(t *testing.T) {
	tests := []string{
		"$branch",
		"${branch}",
		"${branch:-main}",
		"${branch:${empty:-1}:${two+2}}",
	}

	for _, input := range tests {
		l := lexer.New(input)
		p := New(l)
		expr := p.Parse()
		checkParserErrors(t, p)

		expansion, ok := expr.(*ast.ShellExpansion)
		if !ok {
			t.Fatalf("expr is not ast.ShellExpansion. got=%T", expr)
		}
		if expansion.Raw != input {
			t.Fatalf("shell expansion raw = %q, want %q", expansion.Raw, input)
		}
	}
}

func TestParserRejectsUnterminatedShellExpansion(t *testing.T) {
	l := lexer.New(`${branch:-main == "main"`)
	p := New(l)
	p.Parse()

	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors")
	}
}

func TestOperatorPrecedenceParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"!a", "(!a)"},
		{"true", "true"},
		{"false", "false"},
		{"!(true == true)", "(!(true == true))"},
		{"foo.bar.baz == true", "(foo.bar.baz == true)"},
		{"foo.bar == true && bar.baz == false", "((foo.bar == true) && (bar.baz == false))"},
		{"a || b && c", "(a || (b && c))"},
		{"a && b || c", "((a && b) || c)"},
		{"env(env(LLAMAS)) == true", "(env(env(LLAMAS)) == true)"},
		{"a =~ /a/ && b =~ /b/", "((a =~ /a/) && (b =~ /b/))"},
		{"true ? false : true", "(true ? false : true)"},
		{"a || b ? c || d : e || f ? g : h", "((a || b) ? (c || d) : ((e || f) ? g : h))"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		expr := p.Parse()
		checkParserErrors(t, p)

		actual := expr.String()
		if actual != tt.expected {
			t.Errorf("expected=%q, got=%q", tt.expected, actual)
		}
	}
}

func TestBooleanExpression(t *testing.T) {
	tests := []struct {
		input           string
		expectedBoolean bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		expr := p.Parse()
		checkParserErrors(t, p)

		boolean, ok := expr.(*ast.Boolean)
		if !ok {
			t.Fatalf("exp not *ast.Boolean. got=%T", expr)
		}
		if boolean.Value != tt.expectedBoolean {
			t.Errorf("boolean.Value not %t. got=%t", tt.expectedBoolean,
				boolean.Value)
		}
	}
}

func TestNullExpression(t *testing.T) {
	input := "null"

	l := lexer.New(input)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	null, ok := expr.(*ast.Null)
	if !ok {
		t.Fatalf("exp not *ast.Null. got=%T", expr)
	}
	if null.TokenLiteral() != "null" {
		t.Errorf("null.TokenLiteral not %s. got=%s", "null", null.TokenLiteral())
	}
}

func TestCallExpressionParsing(t *testing.T) {
	input := "add(1, 2, 3)"

	l := lexer.New(input)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	exp, ok := expr.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expr is not ast.CallExpression. got=%T", expr)
	}

	if exp.Function != "add" {
		t.Fatalf("Expected function %q, got %q", "add", exp.Function)
	}

	if len(exp.Arguments) != 3 {
		t.Fatalf("wrong length of arguments. got=%d", len(exp.Arguments))
	}

	testLiteralExpression(t, exp.Arguments[0], 1)
	testLiteralExpression(t, exp.Arguments[1], 2)
	testLiteralExpression(t, exp.Arguments[2], 3)
}

func TestCallExpressionParameterParsing(t *testing.T) {
	tests := []struct {
		input         string
		expectedIdent string
		expectedArgs  []string
	}{
		{
			input:         "env()",
			expectedIdent: "env",
			expectedArgs:  []string{},
		},
		{
			input:         "env(1)",
			expectedIdent: "env",
			expectedArgs:  []string{"1"},
		},
		{
			input:         "foo(env(LLAMAS) == 'test' || true)",
			expectedIdent: "foo",
			expectedArgs:  []string{"((env(LLAMAS) == \"test\") || true)"},
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		expr := p.Parse()
		checkParserErrors(t, p)

		exp, ok := expr.(*ast.CallExpression)
		if !ok {
			t.Fatalf("expr is not ast.CallExpression. got=%T", expr)
		}

		if exp.Function != tt.expectedIdent {
			t.Fatalf("Expected function %q, got %q", tt.expectedIdent, exp.Function)
			return
		}

		if len(exp.Arguments) != len(tt.expectedArgs) {
			t.Fatalf("wrong number of arguments. want=%d, got=%d",
				len(tt.expectedArgs), len(exp.Arguments))
		}

		for i, arg := range tt.expectedArgs {
			if exp.Arguments[i].String() != arg {
				t.Errorf("argument %d wrong. want=%q, got=%q", i,
					arg, exp.Arguments[i].String())
			}
		}
	}
}

func TestDottedCallExpressionParsing(t *testing.T) {
	l := lexer.New(`build.env("FOO") == "BAR"`)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expr is not ast.InfixExpression. got=%T", expr)
	}
	call, ok := infix.Left.(*ast.CallExpression)
	if !ok {
		t.Fatalf("left expr is not ast.CallExpression. got=%T", infix.Left)
	}
	if call.Function != "build.env" {
		t.Fatalf("function = %q, want %q", call.Function, "build.env")
	}
	if len(call.Arguments) != 1 {
		t.Fatalf("wrong number of arguments. want=1, got=%d", len(call.Arguments))
	}
	testLiteralExpression(t, call.Arguments[0], "FOO")
}

func TestParsingEmptyArrayLiterals(t *testing.T) {
	input := "[]"

	l := lexer.New(input)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	array, ok := expr.(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("exp not ast.ArrayLiteral. got=%T", expr)
	}

	if len(array.Elements) != 0 {
		t.Errorf("len(array.Elements) not 0. got=%d", len(array.Elements))
	}
}

func TestParsingArrayLiterals(t *testing.T) {
	input := `["llamas", "alpacas"]`

	l := lexer.New(input)
	p := New(l)
	expr := p.Parse()
	checkParserErrors(t, p)

	array, ok := expr.(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("exp not ast.ArrayLiteral. got=%T", expr)
	}

	if len(array.Elements) != 2 {
		t.Fatalf("len(array.Elements) not 2. got=%d", len(array.Elements))
	}

	testIdentifierOrString(t, array.Elements[0], "llamas")
	testIdentifierOrString(t, array.Elements[1], "alpacas")
}

func TestParsingContainsOperators(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{`build.creator.teams includes "deploy"`, "includes"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		expr := p.Parse()
		checkParserErrors(t, p)

		iexpr, ok := expr.(*ast.InfixExpression)
		if !ok {
			t.Fatalf("exp is not ast.InfixExpression. got=%T(%s)", expr, expr)
		}

		if iexpr.Operator != tt.operator {
			t.Fatalf("exp doesn't have expected contains operator. want=%s got=%s", tt.operator, iexpr.Operator)
		}
	}
}

func TestParserRejectsNonServerContainsOperator(t *testing.T) {
	l := lexer.New(`["llamas", "alpacas"] @> "llamas"`)
	p := New(l)
	p.Parse()

	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors")
	}
}

func TestParserRejectsTrailingTokens(t *testing.T) {
	tests := []string{
		`build.creator.teams "deploy"`,
		`build.message !~ /\[skip tests\]/ extra`,
	}

	for _, input := range tests {
		l := lexer.New(input)
		p := New(l)
		p.Parse()

		if len(p.Errors()) == 0 {
			t.Fatalf("expected parser errors for %q", input)
		}
	}
}

func testInfixExpression(t *testing.T, exp ast.Expression, left interface{},
	operator string, right interface{}) bool {

	opExp, ok := exp.(*ast.InfixExpression)
	if !ok {
		t.Errorf("exp is not ast.InfixExpression. got=%T(%s)", exp, exp)
		return false
	}

	if !testLiteralExpression(t, opExp.Left, left) {
		return false
	}

	if opExp.Operator != operator {
		t.Errorf("exp.Operator is not '%s'. got=%q", operator, opExp.Operator)
		return false
	}

	if !testLiteralExpression(t, opExp.Right, right) {
		return false
	}

	return true
}

func testLiteralExpression(t *testing.T, exp ast.Expression, expected interface{}) bool {
	switch v := expected.(type) {
	case int:
		return testIntegerLiteral(t, exp, int64(v))
	case int64:
		return testIntegerLiteral(t, exp, v)
	case string:
		return testIdentifierOrString(t, exp, v)
	case bool:
		return testBooleanLiteral(t, exp, v)
	}
	t.Errorf("type of exp not handled. got=%T", exp)
	return false
}

func testIntegerLiteral(t *testing.T, il ast.Expression, value int64) bool {
	integ, ok := il.(*ast.IntegerLiteral)
	if !ok {
		t.Errorf("il not *ast.IntegerLiteral. got=%T", il)
		return false
	}

	if integ.Value != value {
		t.Errorf("integ.Value not %d. got=%d", value, integ.Value)
		return false
	}

	if integ.TokenLiteral() != fmt.Sprintf("%d", value) {
		t.Errorf("integ.TokenLiteral not %d. got=%s", value,
			integ.TokenLiteral())
		return false
	}

	return true
}

func testIdentifierOrString(t *testing.T, exp ast.Expression, value string) bool {
	switch o := exp.(type) {
	case *ast.Identifier:
		if o.Value != value {
			t.Errorf("ident.Value not %s. got=%s", value, o.Value)
			return false
		}

		if o.TokenLiteral() != value {
			t.Errorf("ident.TokenLiteral not %s. got=%s", value,
				o.TokenLiteral())
			return false
		}

	case *ast.StringLiteral:
		if o.Value != value {
			t.Errorf("ident.Value not %s. got=%s", value, o.Value)
			return false
		}

		if o.TokenLiteral() != value {
			t.Errorf("ident.TokenLiteral not %s. got=%s", value,
				o.TokenLiteral())
			return false
		}

	default:
		t.Errorf("exp not *ast.Identifier or *ast.StringLiteral. got=%T", exp)
		return false
	}

	return true
}

func testBooleanLiteral(t *testing.T, exp ast.Expression, value bool) bool {
	bo, ok := exp.(*ast.Boolean)
	if !ok {
		t.Errorf("exp not *ast.Boolean. got=%T", exp)
		return false
	}

	if bo.Value != value {
		t.Errorf("bo.Value not %t. got=%t", value, bo.Value)
		return false
	}

	if bo.TokenLiteral() != fmt.Sprintf("%t", value) {
		t.Errorf("bo.TokenLiteral not %t. got=%s",
			value, bo.TokenLiteral())
		return false
	}

	return true
}

func checkParserErrors(t *testing.T, p *Parser) {
	t.Helper()

	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}
