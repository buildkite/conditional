package conditional

import (
	"fmt"

	"github.com/buildkite/conditional/ast"
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

func typeCheckExpression(expr ast.Expression, ctx Context) error {
	checker := typeChecker{
		variables: variableTypes(ctx.EntryPoint),
		functions: functionTypes(),
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
		return valueType{kind: kindString}, nil
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
			if err := c.expect(element, kindString); err != nil {
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
		if err := c.expectAny(expr.Right, kindString, kindRegexp, kindNull); err != nil {
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
	return c.checkComparisonTypes(expr.Consequence, expr.Alternative)
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
	leftType, err := c.check(left)
	if err != nil {
		return valueType{kind: kindUnknown}, err
	}

	if leftType.kind == kindNull || leftType.kind == kindUnknown {
		return c.check(right)
	}

	if leftType.enum != nil {
		if err := c.expectAny(right, kindString, kindNull); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		if literal, ok := right.(*ast.StringLiteral); ok && !leftType.enum.includes(literal.Value) {
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

func variableTypes(entryPoint EntryPoint) map[string]valueType {
	variables := map[string]valueType{
		"build.id":                            stringType(),
		"build.state":                         enumValueType("build state", "creating", "started", "running", "scheduled", "blocked", "passed", "failing", "failed", "started_failing", "canceling", "canceled", "skipped", "not_run"),
		"build.fixed":                         boolType(),
		"build.blocked_state":                 enumValueType("build blocked state", "failed", "passed", "running"),
		"build.source":                        enumValueType("build source", "api", "ui", "webhook", "trigger_job", "schedule", "pipeline_trigger"),
		"build.source_event":                  stringType(),
		"build.source_action":                 stringType(),
		"build.branch":                        stringType(),
		"build.tag":                           stringType(),
		"build.message":                       stringType(),
		"build.commit":                        stringType(),
		"build.number":                        numberType(),
		"build.creator.id":                    stringType(),
		"build.creator.name":                  stringType(),
		"build.creator.email":                 stringType(),
		"build.creator.teams":                 stringArrayType(),
		"build.creator.verified":              boolType(),
		"build.author.id":                     stringType(),
		"build.author.name":                   stringType(),
		"build.author.email":                  stringType(),
		"build.author.teams":                  stringArrayType(),
		"build.scm.author.name":               stringType(),
		"build.scm.author.email":              stringType(),
		"build.scm.committer.name":            stringType(),
		"build.scm.committer.email":           stringType(),
		"build.pull_request.id":               stringType(),
		"build.pull_request.base_branch":      stringType(),
		"build.pull_request.draft":            boolType(),
		"build.pull_request.label":            stringType(),
		"build.pull_request.labels":           stringArrayType(),
		"build.pull_request.repository":       stringType(),
		"build.pull_request.repository.fork":  boolType(),
		"build.merge_queue.base_branch":       stringType(),
		"build.merge_queue.base_commit":       stringType(),
		"pipeline.id":                         stringType(),
		"pipeline.name":                       stringType(),
		"pipeline.slug":                       stringType(),
		"pipeline.default_branch":             stringType(),
		"pipeline.repository":                 stringType(),
		"pipeline.started_passing":            boolType(),
		"pipeline.started_failing":            boolType(),
		"pipeline.next_finished_build_exists": boolType(),
		"organization.id":                     stringType(),
		"organization.slug":                   stringType(),
	}

	if stepAllowed(entryPoint) {
		variables["step.id"] = stringType()
		variables["step.key"] = stringType()
		variables["step.type"] = enumValueType("step type", "command", "wait", "input", "trigger", "group")
		variables["step.label"] = stringType()
		variables["step.state"] = enumValueType("step state", "ignored", "waiting_for_dependencies", "ready", "running", "failing", "finished")
		variables["step.outcome"] = enumValueType("step outcome", "neutral", "passed", "soft_failed", "hard_failed", "errored")
	}

	return variables
}

func functionTypes() map[string]functionSignature {
	return map[string]functionSignature{
		"env": {
			args: []valueKind{kindString},
			ret:  stringType(),
		},
		"build.env": {
			args: []valueKind{kindString},
			ret:  stringType(),
		},
	}
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

func identifierName(expr ast.Expression) string {
	if ident, ok := expr.(*ast.Identifier); ok {
		return ident.Value
	}
	return expr.String()
}
