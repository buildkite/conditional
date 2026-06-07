package conditional

import (
	"fmt"

	"github.com/buildkite/conditional/internal/ast"
	"github.com/buildkite/conditional/internal/evaluator"
)

type valueKind string

const (
	kindUnknown     valueKind = "unknown"
	kindString      valueKind = "string"
	kindNumber      valueKind = "number"
	kindBool        valueKind = "boolean"
	kindNull        valueKind = "null"
	kindRegexp      valueKind = "regular expression"
	kindStringArray valueKind = "string array"
)

type enumType struct {
	name   string
	values map[string]struct{}
}

type valueType struct {
	kind valueKind
	enum *enumType
}

type functionSignature struct {
	args []valueKind
	ret  valueType
}

type typeChecker struct {
	variables map[string]valueType
	functions map[string]functionSignature
}

func typeCheckExpression(expr ast.Expression, ctx Context, options optionSet) error {
	checker := typeChecker{
		variables: variableTypes(ctx),
		functions: functionTypes(options),
	}

	got, err := checker.check(expr)
	if err != nil {
		return err
	}
	if got.kind != kindBool && got.kind != kindUnknown {
		return &Error{
			Kind:    ErrorKindResult,
			Message: fmt.Sprintf("expected boolean result, got %s", got.describe()),
		}
	}
	return nil
}

func (c typeChecker) check(expr ast.Expression) (valueType, error) {
	switch expr := expr.(type) {
	case *ast.Boolean:
		return valueType{kind: kindBool}, nil
	case *ast.Null:
		return valueType{kind: kindNull}, nil
	case *ast.IntegerLiteral:
		return valueType{kind: kindNumber}, nil
	case *ast.StringLiteral:
		return valueType{kind: kindString}, nil
	case *ast.ShellExpansion:
		return stringType(), nil
	case *ast.Regexp:
		return valueType{kind: kindRegexp}, nil
	case *ast.Identifier:
		typ, ok := c.variables[expr.Value]
		if !ok {
			return valueType{kind: kindUnknown}, validationError("`%s` is not a variable", expr.Value)
		}
		return typ, nil
	case *ast.PrefixExpression:
		if expr.Operator != "!" {
			return valueType{kind: kindUnknown}, validationError("`%s` is not a prefix operator", expr.Operator)
		}
		if err := c.expect(expr.Right, kindBool); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		return valueType{kind: kindBool}, nil
	case *ast.InfixExpression:
		return c.checkInfix(expr)
	case *ast.ConditionalExpression:
		return c.checkConditional(expr)
	case *ast.CallExpression:
		return c.checkCall(expr)
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if err := c.expectArrayElement(element); err != nil {
				return valueType{kind: kindUnknown}, err
			}
		}
		return valueType{kind: kindStringArray}, nil
	default:
		return valueType{kind: kindUnknown}, validationError("unsupported expression type %T", expr)
	}
}

func (c typeChecker) checkInfix(expr *ast.InfixExpression) (valueType, error) {
	switch expr.Operator {
	case "=~", "!~":
		if err := c.expectAny(expr.Left, kindString, kindNull); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		if err := c.expect(expr.Right, kindRegexp); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		return valueType{kind: kindBool}, nil
	case "includes":
		if err := c.expectAny(expr.Left, kindStringArray, kindNull); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		if err := c.expectIncludesRight(expr.Right); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		return valueType{kind: kindBool}, nil
	case "&&", "||":
		if err := c.expect(expr.Left, kindBool); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		if err := c.expect(expr.Right, kindBool); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		return valueType{kind: kindBool}, nil
	case "==", "!=":
		if _, err := c.checkComparisonTypes(expr.Left, expr.Right); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		return valueType{kind: kindBool}, nil
	default:
		return valueType{kind: kindUnknown}, validationError("`%s` is not a comparison operator", expr.Operator)
	}
}

func (c typeChecker) checkConditional(expr *ast.ConditionalExpression) (valueType, error) {
	if err := c.expect(expr.Condition, kindBool); err != nil {
		return valueType{kind: kindUnknown}, err
	}
	return c.checkCompatibleTypes(expr.Consequence, expr.Alternative)
}

func (c typeChecker) checkCall(expr *ast.CallExpression) (valueType, error) {
	signature, ok := c.functions[expr.Function]
	if !ok {
		return valueType{kind: kindUnknown}, validationError("`%s` is not a function", expr.Function)
	}
	if len(expr.Arguments) != len(signature.args) {
		return valueType{kind: kindUnknown}, validationError(
			"wrong number of arguments for `%s`: got %d, want %d",
			expr.Function,
			len(expr.Arguments),
			len(signature.args),
		)
	}
	for i, arg := range expr.Arguments {
		if err := c.expect(arg, signature.args[i]); err != nil {
			return valueType{kind: kindUnknown}, err
		}
	}
	return signature.ret, nil
}

func (c typeChecker) checkComparisonTypes(left, right ast.Expression) (valueType, error) {
	return c.checkCompatibleTypes(left, right)
}

func (c typeChecker) checkCompatibleTypes(left, right ast.Expression) (valueType, error) {
	leftType, err := c.check(left)
	if err != nil {
		return valueType{kind: kindUnknown}, err
	}

	rightType, err := c.check(right)
	if err != nil {
		return valueType{kind: kindUnknown}, err
	}
	if leftType.kind == kindNull || leftType.kind == kindUnknown {
		return rightType, nil
	}
	if rightType.kind == kindNull || rightType.kind == kindUnknown {
		return leftType, nil
	}
	if leftType.kind == kindStringArray || rightType.kind == kindStringArray {
		if leftType.kind != rightType.kind {
			return valueType{kind: kindUnknown}, validationError("unexpected type: expected %s but found %s", leftType.describe(), rightType.describe())
		}
		return leftType, nil
	}
	if leftType.enum != nil {
		if err := c.expectAny(right, kindString, kindNull); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		if literal, ok := staticStringLiteral(right); ok && !leftType.enum.includes(literal.Value) {
			return valueType{kind: kindUnknown}, validationError("%q is not a valid `%s`", literal.Value, identifierName(left))
		}
		return leftType, nil
	}

	if err := c.expectAny(right, leftType.kind, kindNull); err != nil {
		return valueType{kind: kindUnknown}, err
	}
	return leftType, nil
}

func (c typeChecker) expect(expr ast.Expression, expected valueKind) error {
	return c.expectAny(expr, expected)
}

func (c typeChecker) expectAny(expr ast.Expression, expected ...valueKind) error {
	actual, err := c.check(expr)
	if err != nil {
		return err
	}
	if actual.kind == kindUnknown {
		return nil
	}
	for _, expectedKind := range expected {
		if actual.enum == nil && actual.kind == expectedKind {
			return nil
		}
	}

	return validationError("unexpected type: expected %s but found %s", describeKinds(expected), actual.describe())
}

func (c typeChecker) expectArrayElement(expr ast.Expression) error {
	actual, err := c.check(expr)
	if err != nil {
		return err
	}
	if actual.kind == kindUnknown {
		return nil
	}
	if actual.kind == kindString && actual.enum == nil {
		return nil
	}
	return validationError("unexpected type: expected string but found %s", actual.describe())
}

func (c typeChecker) expectIncludesRight(expr ast.Expression) error {
	actual, err := c.check(expr)
	if err != nil {
		return err
	}
	if actual.kind == kindUnknown {
		return nil
	}
	switch actual.kind {
	case kindString:
		if actual.enum == nil {
			return nil
		}
	case kindRegexp, kindNull:
		return nil
	}
	return validationError("unexpected type: expected string, regular expression, or null but found %s", actual.describe())
}

func variableTypes(ctx Context) map[string]valueType {
	definitions := assignmentDefinitions(ctx)
	variables := make(map[string]valueType, len(definitions))
	for _, definition := range definitions {
		variables[definition.name] = definition.typ
	}

	return variables
}

func functionTypes(options optionSet) map[string]functionSignature {
	functions := map[string]functionSignature{
		"env": {
			args: []valueKind{kindString},
			ret:  stringType(),
		},
		"build.env": {
			args: []valueKind{kindString},
			ret:  stringType(),
		},
	}
	for name, function := range options.functions {
		signature, err := function.signature()
		if err != nil {
			continue
		}
		functions[name] = signature
	}

	return functions
}

func stringType() valueType {
	return valueType{kind: kindString}
}

func numberType() valueType {
	return valueType{kind: kindNumber}
}

func boolType() valueType {
	return valueType{kind: kindBool}
}

func stringArrayType() valueType {
	return valueType{kind: kindStringArray}
}

func enumValueType(name string, values ...string) valueType {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return valueType{
		kind: kindString,
		enum: &enumType{name: name, values: set},
	}
}

func stepString(step *Step, value func(*Step) *string) *string {
	if step == nil {
		return nil
	}
	return value(step)
}

func (t valueType) describe() string {
	if t.enum != nil {
		return t.enum.name + " enumeration value"
	}
	return string(t.kind)
}

func (e enumType) includes(value string) bool {
	_, ok := e.values[value]
	return ok
}

func validationError(format string, args ...any) *Error {
	return &Error{Kind: ErrorKindValidation, Message: fmt.Sprintf(format, args...)}
}

func describeKinds(kinds []valueKind) string {
	if len(kinds) == 1 {
		return string(kinds[0])
	}

	out := ""
	for i, kind := range kinds {
		switch {
		case i == 0:
			out = string(kind)
		case i == len(kinds)-1:
			out += " or " + string(kind)
		default:
			out += ", " + string(kind)
		}
	}
	return out
}

func runtimeStringLiteral(literal *ast.StringLiteral) bool {
	raw := literal.Token.Raw
	if raw == "" {
		raw = literal.Value
	}
	return literal.Token.Flags == `"` && evaluator.ContainsShellExpansion(raw)
}

func staticStringLiteral(expr ast.Expression) (*ast.StringLiteral, bool) {
	literal, ok := expr.(*ast.StringLiteral)
	if !ok || runtimeStringLiteral(literal) {
		return nil, false
	}
	return literal, true
}

func identifierName(expr ast.Expression) string {
	if ident, ok := expr.(*ast.Identifier); ok {
		return ident.Value
	}
	return expr.String()
}
