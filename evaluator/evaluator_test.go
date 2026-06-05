package evaluator

import (
	"testing"

	"github.com/buildkite/conditional/lexer"
	"github.com/buildkite/conditional/object"
	"github.com/buildkite/conditional/parser"
)

func TestEvalBooleanExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{`"a" == "a"`, true},
		{`"a" == "b"`, false},
		{`"a" != "a"`, false},
		{`"a" != "b"`, true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"null == null", true},
		{"null != null", false},
		{"'a' =~ /a/", true},
		{"'b' !~ /a/", true},
		{`/skip/i == /skip/`, false},
		{`/skip/i != /skip/`, true},
		{`/skip/i == /skip/i`, true},
		{`"features/foo" =~ /^features\//`, true},
		{`"feature/release-123" =~ /\/release-123$/`, true},
		{`"v1.0" =~ /^v[0-9]+\.0$/`, true},
		{`"v123" =~ /^v[[:digit:]]+$/`, true},
		{`"price $5" =~ /price \$[0-9]+/`, true},
		{`"$" =~ /\$/`, true},
		{`"price $" =~ /price \$/`, true},
		{`"fee$" =~ /fee\$/`, true},
		{`"fee" =~ /fee\$/`, false},
		{`"main" =~ /^(main$|release\/.*$)/`, true},
		{`"release/foo" =~ /^(main$|release\/.*$)/`, true},
		{`"[SKIP TESTS]" =~ /\[skip tests\]/i`, true},
		{`"[SKIP TESTS]" !~ /\[skip tests\]/i`, false},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestBangOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestEvalBooleanObjectComparison(t *testing.T) {
	scope := object.Struct{
		"build": object.Struct{
			"pull_request": object.Struct{
				"draft": &object.Boolean{Value: false},
			},
		},
	}

	evaluated := testEvalWithScope(`build.pull_request.draft == false`, scope)
	testBooleanObject(t, evaluated, true)
}

func TestEvalNullComparison(t *testing.T) {
	scope := object.Struct{
		"build": object.Struct{
			"tag": &object.Null{},
		},
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{`build.tag == null`, true},
		{`build.tag != null`, false},
		{`build.tag == "v1.0.0"`, false},
		{`build.tag != "v1.0.0"`, true},
	}

	for _, tt := range tests {
		evaluated := testEvalWithScope(tt.input, scope)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestLogicalOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true && true", true},
		{"true && false", false},
		{"false || true", true},
		{"false || false", false},
		{"false && missing.value", false},
		{"true || missing.value", true},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestCallOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"env('test') == 'test'", true},
		{"env(foo('a', 'b')) == 'test'", true},
	}

	env := object.Struct{
		`env`: object.Function(func(args []object.Object) object.Object {
			return args[0]
		}),
		`foo`: object.Function(func(args []object.Object) object.Object {
			return &object.String{Value: "test"}
		}),
	}

	for _, tt := range tests {
		evaluated := testEvalWithScope(tt.input, env)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestDotOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"foo.bar.baz == 'test'", true},
	}

	scope := object.Struct{
		`foo`: object.Struct{
			`bar`: object.Struct{
				`baz`: &object.String{Value: "test"},
			},
		},
	}

	for _, tt := range tests {
		evaluated := testEvalWithScope(tt.input, scope)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestDotOperatorFailsOnMissingStructProperty(t *testing.T) {
	obj := testEvalWithScope(`foo.bar`, object.Struct{
		`foo`: object.Struct{},
	})

	result, ok := obj.(*object.Error)
	if !ok {
		t.Fatalf("result is not an error. got=%T (%+v)", obj, obj)
	}

	if result.Message != `struct has no property "bar"` {
		t.Fatalf("bad error message: %v", result.Message)
	}
}

func TestContainsOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`["llamas","alpacas"] includes 'alpacas'`, true},
		{`["llamas","alpacas"] includes 'sheep'`, false},
		{`["llamas","alpacas"] @> 'alpacas'`, true},
		{`["llamas","alpacas"] @> 'sheep'`, false},
		{`[1,2,3] @> 2`, true},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestContainsOperatorWithScopeArray(t *testing.T) {
	scope := object.Struct{
		"build": object.Struct{
			"creator": object.Struct{
				"teams": &object.Array{Elements: []object.Object{
					&object.String{Value: "deploy"},
					&object.String{Value: "platform"},
				}},
			},
		},
	}

	evaluated := testEvalWithScope(`build.creator.teams includes "deploy"`, scope)
	testBooleanObject(t, evaluated, true)
}

func testEval(input string) object.Object {
	return testEvalWithScope(input, object.Struct{})
}

func testEvalWithScope(input string, scope Scope) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	expr := p.Parse()
	return Eval(expr, scope)
}

func testBooleanObject(t *testing.T, obj object.Object, expected bool) bool {
	result, ok := obj.(*object.Boolean)
	if !ok {
		t.Errorf("object is not Boolean. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t",
			result.Value, expected)
		return false
	}
	return true
}
