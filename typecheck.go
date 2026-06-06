package conditional

import (
	"fmt"

	"github.com/buildkite/conditional/ast"
	"github.com/buildkite/conditional/evaluator"
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
	kind     valueKind
	enum     *enumType
	nullable bool
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
		variables: variableTypes(ctx),
		functions: functionTypes(),
	}

	got, err := checker.check(expr)
	if err != nil {
		return err
	}
	if (got.kind != kindBool && got.kind != kindUnknown) || got.nullable {
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
		if err := c.expectAny(expr.Right, kindBool, kindNull); err != nil {
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
		if logicalExpressionShortCircuits(expr) {
			return valueType{kind: kindBool}, nil
		}
		rightChecker := c
		if name, ok := nonNullGuardForRHS(expr.Operator, expr.Left); ok {
			rightChecker = c.withNonNullableVariable(name)
		}
		if err := rightChecker.expect(expr.Right, kindBool); err != nil {
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
	consequenceChecker := c
	alternativeChecker := c
	if name, consequenceIsNonNull, ok := nonNullGuardForConditional(expr.Condition); ok {
		if consequenceIsNonNull {
			consequenceChecker = c.withNonNullableVariable(name)
		} else {
			alternativeChecker = c.withNonNullableVariable(name)
		}
	}
	return consequenceChecker.checkCompatibleTypesWith(alternativeChecker, expr.Consequence, expr.Alternative, true)
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
	return c.checkCompatibleTypes(left, right, false)
}

func (c typeChecker) checkCompatibleTypes(left, right ast.Expression, allowArrays bool) (valueType, error) {
	return c.checkCompatibleTypesWith(c, left, right, allowArrays)
}

func (c typeChecker) checkCompatibleTypesWith(rightChecker typeChecker, left, right ast.Expression, allowArrays bool) (valueType, error) {
	leftType, err := c.check(left)
	if err != nil {
		return valueType{kind: kindUnknown}, err
	}

	rightType, err := rightChecker.check(right)
	if err != nil {
		return valueType{kind: kindUnknown}, err
	}
	if !allowArrays && leftType.kind == kindStringArray {
		return valueType{kind: kindUnknown}, validationError("unexpected type: expected scalar comparison operand but found %s", leftType.describe())
	}
	if !allowArrays && rightType.kind == kindStringArray {
		return valueType{kind: kindUnknown}, validationError("unexpected type: expected scalar comparison operand but found %s", rightType.describe())
	}
	if leftType.kind == kindNull || leftType.kind == kindUnknown {
		return rightType.withNull(), nil
	}
	if rightType.kind == kindNull || rightType.kind == kindUnknown {
		return leftType.withNull(), nil
	}
	if leftType.kind == kindStringArray || rightType.kind == kindStringArray {
		if leftType.kind != rightType.kind {
			return valueType{kind: kindUnknown}, validationError("unexpected type: expected %s but found %s", leftType.describe(), rightType.describe())
		}
		return leftType.withNullabilityFrom(rightType), nil
	}
	if leftType.enum != nil {
		if rightType.enum != nil {
			if !leftType.enum.compatible(rightType.enum) {
				return valueType{kind: kindUnknown}, validationError("unexpected type: expected %s but found %s", leftType.describe(), rightType.describe())
			}
			return leftType.withNullabilityFrom(rightType), nil
		}
		if err := c.expectAny(right, kindString, kindNull); err != nil {
			return valueType{kind: kindUnknown}, err
		}
		if literal, ok := staticStringLiteral(right); ok && !leftType.enum.includes(literal.Value) {
			return valueType{kind: kindUnknown}, validationError("%q is not a valid `%s`", literal.Value, identifierName(left))
		}
		return leftType.withNullabilityFrom(rightType), nil
	}

	if rightType.enum != nil && leftType.kind == kindString {
		if literal, ok := staticStringLiteral(left); ok && !rightType.enum.includes(literal.Value) {
			return valueType{kind: kindUnknown}, validationError("%q is not a valid `%s`", literal.Value, identifierName(right))
		}
		return rightType.withNullabilityFrom(leftType), nil
	}

	if err := c.expectAny(right, leftType.kind, kindNull); err != nil {
		return valueType{kind: kindUnknown}, err
	}
	return leftType.withNullabilityFrom(rightType), nil
}

func (c typeChecker) withNonNullableVariable(name string) typeChecker {
	typ, ok := c.variables[name]
	if !ok || !typ.nullable {
		return c
	}

	variables := make(map[string]valueType, len(c.variables))
	for key, value := range c.variables {
		variables[key] = value
	}
	typ.nullable = false
	variables[name] = typ
	c.variables = variables
	return c
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
	allowsNull := containsKind(expected, kindNull)
	for _, expectedKind := range expected {
		if actual.enum == nil && actual.kind == expectedKind {
			if actual.nullable && !allowsNull {
				continue
			}
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
	if actual.kind == kindString && !actual.nullable {
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
	case kindString, kindRegexp, kindNull:
		return nil
	}
	return validationError("unexpected type: expected string, regular expression, or null but found %s", actual.describe())
}

func variableTypes(ctx Context) map[string]valueType {
	variables := map[string]valueType{
		"build.id":                            stringType(),
		"build.state":                         enumValueType("build state", "creating", "started", "running", "scheduled", "blocked", "passed", "failing", "failed", "started_failing", "canceling", "canceled", "skipped", "not_run"),
		"build.fixed":                         boolTypeFor(ctx.Build.Fixed),
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
		"build.creator.verified":              boolTypeFor(ctx.Build.Creator.Verified),
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
		"build.pull_request.draft":            boolTypeFor(ctx.Build.PullRequest.Draft),
		"build.pull_request.label":            stringType(),
		"build.pull_request.labels":           stringArrayType(),
		"build.pull_request.repository":       stringType(),
		"build.pull_request.repository.fork":  boolTypeFor(ctx.Build.PullRequest.RepositoryFork),
		"build.merge_queue.base_branch":       stringType(),
		"build.merge_queue.base_commit":       stringType(),
		"pipeline.id":                         stringType(),
		"pipeline.name":                       stringType(),
		"pipeline.slug":                       stringType(),
		"pipeline.default_branch":             stringType(),
		"pipeline.repository":                 stringType(),
		"pipeline.started_passing":            boolTypeFor(ctx.Pipeline.StartedPassing),
		"pipeline.started_failing":            boolTypeFor(ctx.Pipeline.StartedFailing),
		"pipeline.next_finished_build_exists": boolTypeFor(ctx.Pipeline.NextFinishedBuildExists),
		"organization.id":                     stringType(),
		"organization.slug":                   stringType(),
	}

	if stepAllowed(ctx.EntryPoint) {
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

func nullableBoolType() valueType {
	return valueType{kind: kindBool, nullable: true}
}

func boolTypeFor(value *bool) valueType {
	if value == nil {
		return nullableBoolType()
	}
	return boolType()
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
		if t.nullable {
			return "nullable " + t.enum.name + " enumeration value"
		}
		return t.enum.name + " enumeration value"
	}
	if t.nullable {
		return "nullable " + string(t.kind)
	}
	return string(t.kind)
}

func (t valueType) withNull() valueType {
	if t.kind == kindUnknown {
		return t
	}
	t.nullable = true
	return t
}

func (t valueType) withNullabilityFrom(other valueType) valueType {
	if other.kind == kindNull || other.nullable {
		return t.withNull()
	}
	return t
}

func (e enumType) includes(value string) bool {
	_, ok := e.values[value]
	return ok
}

func (e enumType) compatible(other *enumType) bool {
	return other != nil && e.name == other.name
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

func containsKind(kinds []valueKind, target valueKind) bool {
	for _, kind := range kinds {
		if kind == target {
			return true
		}
	}
	return false
}

func runtimeStringLiteral(literal *ast.StringLiteral) bool {
	return literal.Token.Flags == `"` && evaluator.ContainsShellExpansion(literal.Value)
}

func staticStringLiteral(expr ast.Expression) (*ast.StringLiteral, bool) {
	literal, ok := expr.(*ast.StringLiteral)
	if !ok || runtimeStringLiteral(literal) {
		return nil, false
	}
	return literal, true
}

func logicalExpressionShortCircuits(expr *ast.InfixExpression) bool {
	left, ok := expr.Left.(*ast.Boolean)
	if !ok {
		return false
	}

	return (expr.Operator == "&&" && !left.Value) || (expr.Operator == "||" && left.Value)
}

func nonNullGuardForRHS(operator string, left ast.Expression) (string, bool) {
	guard, ok := left.(*ast.InfixExpression)
	if !ok {
		return "", false
	}
	switch {
	case operator == "&&" && guard.Operator == "!=":
		return nullComparedIdentifier(guard.Left, guard.Right)
	case operator == "||" && guard.Operator == "==":
		return nullComparedIdentifier(guard.Left, guard.Right)
	default:
		return "", false
	}
}

func nonNullGuardForConditional(condition ast.Expression) (name string, consequenceIsNonNull bool, ok bool) {
	guard, ok := condition.(*ast.InfixExpression)
	if !ok {
		return "", false, false
	}
	name, ok = nullComparedIdentifier(guard.Left, guard.Right)
	if !ok {
		return "", false, false
	}
	switch guard.Operator {
	case "!=":
		return name, true, true
	case "==":
		return name, false, true
	default:
		return "", false, false
	}
}

func nullComparedIdentifier(left, right ast.Expression) (string, bool) {
	if _, ok := right.(*ast.Null); ok {
		if ident, ok := left.(*ast.Identifier); ok {
			return ident.Value, true
		}
	}
	if _, ok := left.(*ast.Null); ok {
		if ident, ok := right.(*ast.Identifier); ok {
			return ident.Value, true
		}
	}
	return "", false
}

func identifierName(expr ast.Expression) string {
	if ident, ok := expr.(*ast.Identifier); ok {
		return ident.Value
	}
	return expr.String()
}
