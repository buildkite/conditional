package evaluator

import (
	"testing"

	"github.com/buildkite/conditional/internal/lexer"
	"github.com/buildkite/conditional/internal/object"
	"github.com/buildkite/conditional/internal/parser"
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
		"build.pull_request.draft": &object.Boolean{Value: false},
	}

	evaluated := testEvalWithScope(`build.pull_request.draft == false`, scope)
	testBooleanObject(t, evaluated, true)
}

func TestEvalNullComparison(t *testing.T) {
	scope := object.Struct{
		"build.tag": &object.Null{},
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
		{"env(env('name')) == 'test'", true},
	}

	env := object.Struct{
		`env`: object.Function(func(args []object.Object) object.Object {
			name := args[0].(*object.String).Value
			values := map[string]string{
				"name": "test",
				"test": "test",
			}
			return &object.String{Value: values[name]}
		}),
	}

	for _, tt := range tests {
		evaluated := testEvalWithScope(tt.input, env)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestFlatDottedIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"foo.bar.baz == 'test'", true},
	}

	scope := object.Struct{
		`foo.bar.baz`: &object.String{Value: "test"},
	}

	for _, tt := range tests {
		evaluated := testEvalWithScope(tt.input, scope)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestNestedDottedIdentifierIsNotResolved(t *testing.T) {
	scope := object.Struct{
		"build": object.Struct{
			"message": &object.String{Value: "deploy"},
		},
	}

	evaluated := testEvalWithScope(`build.message == "deploy"`, scope)
	result, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("result is not an error. got=%T (%+v)", evaluated, evaluated)
	}
	if result.Message != `identifier not found: build.message` {
		t.Fatalf("bad error message: %v", result.Message)
	}
}

func TestFlatDottedIdentifierIgnoresNestedStructs(t *testing.T) {
	scope := object.Struct{
		"build.message": &object.String{Value: "flat"},
		"build": object.Struct{
			"message": &object.String{Value: "nested"},
		},
	}

	evaluated := testEvalWithScope(`build.message == "flat"`, scope)
	testBooleanObject(t, evaluated, true)
}

func TestFlatDottedIdentifierFailsOnMissingAssignment(t *testing.T) {
	obj := testEvalWithScope(`foo.bar`, object.Struct{})

	result, ok := obj.(*object.Error)
	if !ok {
		t.Fatalf("result is not an error. got=%T (%+v)", obj, obj)
	}

	if result.Message != `identifier not found: foo.bar` {
		t.Fatalf("bad error message: %v", result.Message)
	}
}

func TestNestedDottedFunctionIsNotResolved(t *testing.T) {
	scope := object.Struct{
		"build": object.Struct{
			"env": object.Function(func(args []object.Object) object.Object {
				return &object.String{Value: "from-nested"}
			}),
		},
	}

	evaluated := testEvalWithScope(`build.env("FOO") == "from-nested"`, scope)
	result, ok := evaluated.(*object.Error)
	if !ok {
		t.Fatalf("result is not an error. got=%T (%+v)", evaluated, evaluated)
	}
	if result.Message != `function not defined: build.env` {
		t.Fatalf("bad error message: %v", result.Message)
	}
}

func TestIncludesOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`["llamas","alpacas"] includes 'alpacas'`, true},
		{`["llamas","alpacas"] includes 'sheep'`, false},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestIncludesOperatorWithScopeArray(t *testing.T) {
	scope := object.Struct{
		"build.creator.teams": &object.Array{Elements: []object.Object{
			&object.String{Value: "deploy"},
			&object.String{Value: "platform"},
		}},
	}

	evaluated := testEvalWithScope(`build.creator.teams includes "deploy"`, scope)
	testBooleanObject(t, evaluated, true)
}

func TestConditionalExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`1 == 2 ? 3 == 4 : 5 == 5`, true},
		{`1 == 1 ? 3 == 4 : 5 == 5`, false},
		{`false ? true : false ? true : true`, true},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestDoubleQuotedShellExpansionWithoutEnvironmentScopeRemainsLiteral(t *testing.T) {
	evaluated := testEval(`"${branch}" == "${branch}"`)
	testBooleanObject(t, evaluated, true)
}

func TestContainsShellExpansionConsumesStringEscapes(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected bool
	}{
		{
			name:     "unescaped variable expands",
			raw:      `${branch}`,
			expected: true,
		},
		{
			name:     "escaped dollar stays literal",
			raw:      `\$branch`,
			expected: false,
		},
		{
			name:     "dollar after escaped slash expands",
			raw:      `\\$branch`,
			expected: true,
		},
		{
			name:     "escaped dollar after escaped slash stays literal",
			raw:      `\\\$branch`,
			expected: false,
		},
		{
			name:     "hex escaped dollar stays literal",
			raw:      `\x24branch`,
			expected: false,
		},
	}

	for _, tt := range tests {
		if got := ContainsShellExpansion(tt.raw); got != tt.expected {
			t.Errorf("%s: ContainsShellExpansion(%q) = %t, want %t", tt.name, tt.raw, got, tt.expected)
		}
	}
}

func TestShellFallbackStringGrammar(t *testing.T) {
	scope := shellTestScope{
		Struct: object.Struct{},
		env: map[string]string{
			"branch": "main",
		},
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{`${missing-\x41\svalue} == "A value"`, true},
		{`${missing-"$branch"} == "main"`, true},
		{`${missing-"}"} == "}"`, true},
		{`${missing-'quoted fallback'} == "quoted fallback"`, true},
		{`${missing-\q} == "q"`, true},
	}

	for _, tt := range tests {
		evaluated := testEvalWithScope(tt.input, scope)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestShellStringEscapesUseBytes(t *testing.T) {
	got, err := evalShellString(`\xff\377`, shellTestScope{})
	if err != nil {
		t.Fatalf("evalShellString returned error: %v", err)
	}

	want := string([]byte{0xff, 0xff})
	if got != want {
		t.Fatalf("evalShellString bytes = %v, want %v", []byte(got), []byte(want))
	}
}

func TestShellStringEscapesRejectOutOfRangeOctal(t *testing.T) {
	if _, err := evalShellString(`\400`, shellTestScope{}); err == nil {
		t.Fatal("evalShellString did not reject out-of-range octal escape")
	}
}

type shellTestScope struct {
	object.Struct
	env map[string]string
}

func (s shellTestScope) LookupEnv(key string) (string, bool) {
	value, ok := s.env[key]
	return value, ok
}

func testEval(input string) object.Object {
	return testEvalWithScope(input, object.Struct{})
}

func testEvalWithScope(input string, scope Scope) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	expr := p.Parse()
	if len(p.Errors()) > 0 {
		return &object.Error{Message: p.Errors()[0]}
	}
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
