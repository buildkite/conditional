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
		{"'a' =~ /a/", true},
		{"'b' !~ /a/", true},
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

func TestCallOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"env('test') == 'test'", true},
		{"env(foo('a', 'b')) == 'test'", true},
	}

	env := &object.Struct{
		`env`: &object.Function{Fn: func(args []object.Object) object.Object {
			return args[0]
		}},
		`foo`: &object.Function{Fn: func(args []object.Object) object.Object {
			return &object.String{Value: "test"}
		}},
	}

	for _, tt := range tests {
		evaluated := testEvalWithEnv(tt.input, env)
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

	env := &object.Struct{
		`foo`: &object.Struct{
			`bar`: &object.Struct{
				`baz`: &object.String{Value: "test"},
			},
		},
	}

	for _, tt := range tests {
		evaluated := testEvalWithEnv(tt.input, env)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestDotOperatorWithReflectFromInterfaceMap(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"foo.bar.baz == 'test'", true},
		{"foo.bar.another", true},
		{"foo.bar.andanint == 24", true},
	}

	env := object.NewReflectMap(map[string]interface{}{
		`foo`: map[string]interface{}{
			`bar`: map[string]interface{}{
				`baz`:      "test",
				`another`:  true,
				`andanint`: 24,
			},
		},
	})

	for _, tt := range tests {
		evaluated := testEvalWithEnv(tt.input, env)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestDotOperatorWithReflectFromStruct(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"foo.bar.baz == 'test'", true},
		{"foo.bar.another", true},
		{"foo.bar.andanint == 24", true},
	}

	var obj struct {
		Foo struct {
			Bar struct {
				Baz      string
				Another  bool
				AndAnInt int
			}
		}
	}

	obj.Foo.Bar.Baz = "test"
	obj.Foo.Bar.Another = true
	obj.Foo.Bar.AndAnInt = 24

	env := object.NewReflectMap(obj)

	for _, tt := range tests {
		evaluated := testEvalWithEnv(tt.input, env)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestDotOperatorFailsOnMissingStructProperty(t *testing.T) {
	obj := testEvalWithEnv(`foo.bar`, &object.Struct{
		`foo`: &object.Struct{},
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
		{`["llamas","alpacas"] @> 'alpacas'`, true},
		{`["llamas","alpacas"] @> 'sheep'`, false},
		{`[1,2,3] @> 2`, true},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func testEval(input string) object.Object {
	return testEvalWithEnv(input, &object.Struct{})
}

func testEvalWithEnv(input string, env object.Map) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	expr := p.Parse()
	return Eval(expr, env)
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Errorf("object is not Integer. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d",
			result.Value, expected)
		return false
	}

	return true
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

func testNullObject(t *testing.T, obj object.Object) bool {
	if obj != NULL {
		t.Errorf("object is not NULL. got=%T (%+v)", obj, obj)
		return false
	}
	return true
}
