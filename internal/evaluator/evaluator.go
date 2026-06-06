package evaluator

import (
	"fmt"

	"github.com/buildkite/conditional/internal/ast"
	"github.com/buildkite/conditional/internal/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

type Scope interface {
	Get(key string) (object.Object, bool)
}

// Eval an ast.Node (either a literal or an expression), with a scope struct
func Eval(node ast.Node, scope Scope) object.Object {
	// defer untrace(trace("Eval", fmt.Sprintf("%T `%s`", node, node.String())))

	switch node := node.(type) {

	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}

	case *ast.StringLiteral:
		return evalStringLiteral(node.Value, node.Token.Raw, node.Token.Flags, scope)

	case *ast.Regexp:
		return &object.Regexp{Regexp: node.Regexp, Flags: node.Flags}

	case *ast.ShellExpansion:
		return evalShellExpansion(node.Raw, scope)

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.Null:
		return NULL

	case *ast.PrefixExpression:
		right := Eval(node.Right, scope)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)

	case *ast.InfixExpression:
		left := Eval(node.Left, scope)
		if isError(left) {
			return left
		}

		if node.Operator == "&&" || node.Operator == "||" {
			return evalLogicalExpression(node.Operator, left, node.Right, scope)
		}

		var right object.Object
		right = Eval(node.Right, scope)
		if isError(right) {
			return right
		}

		return evalInfixExpression(node.Operator, left, right)

	case *ast.ConditionalExpression:
		return evalConditionalExpression(node, scope)

	case *ast.Identifier:
		return evalIdentifier(node, scope)

	case *ast.CallExpression:
		args := evalExpressions(node.Arguments, scope)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}

		obj, ok := resolveScopedName(node.Function, scope)
		if !ok {
			return newError("function not defined: %s", node.Function)
		}

		return applyFunction(obj, args)

	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, scope)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}

	default:
		return newError("unhandled type: %T", node)
	}
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalConditionalExpression(node *ast.ConditionalExpression, scope Scope) object.Object {
	condition := Eval(node.Condition, scope)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(node.Consequence, scope)
	}
	return Eval(node.Alternative, scope)
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	// defer untrace(trace("applyFunction", args))

	switch fn := fn.(type) {
	case object.Function:
		ret := fn(args)
		// tracePrint(fmt.Sprintf("RETURN: %+v", ret))
		return ret

	default:
		return newError("not a function: %s", fn.Type())
	}
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	// defer untrace(trace("evalPrefixExpression", operator, right))

	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	// defer untrace(trace("evalInfixExpression", operator, left, right))

	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.REGEXP_OBJ:
		return evalStringRegexpInfixExpression(operator, left, right)
	case left.Type() == object.NULL_OBJ && right.Type() == object.REGEXP_OBJ:
		return evalNullRegexpInfixExpression(operator, left, right)
	case left.Type() == object.ARRAY_OBJ:
		return evalArrayInfixExpression(operator, left, right)
	case left.Type() == object.NULL_OBJ && operator == "includes":
		return FALSE
	case operator == "==":
		return nativeBoolToBooleanObject(left.Type() == right.Type() && left.Equals(right))
	case operator == "!=":
		return nativeBoolToBooleanObject(left.Type() != right.Type() || !left.Equals(right))
	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s",
			left.Type(), operator, right.Type())
	default:
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalLogicalExpression(operator string, left object.Object, rightExp ast.Expression, scope Scope) object.Object {
	switch operator {
	case "&&":
		if !isTruthy(left) {
			return left
		}
	case "||":
		if isTruthy(left) {
			return left
		}
	}

	right := Eval(rightExp, scope)
	if isError(right) {
		return right
	}

	return right
}

func evalBangOperatorExpression(right object.Object) object.Object {
	// defer untrace(trace("evalBangOperatorExpression", right))

	switch right := right.(type) {
	case *object.Boolean:
		return nativeBoolToBooleanObject(!right.Value)
	case *object.Null:
		return TRUE
	default:
		return FALSE
	}
}

func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	default:
		return true
	}
}

func evalIntegerInfixExpression(operator string, left, right object.Object) object.Object {
	// defer untrace(trace("evalIntegerInfixExpression", operator, left, right))

	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(operator string, left, right object.Object) object.Object {
	// defer untrace(trace("evalStringInfixExpression", operator, left, right))

	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	switch operator {
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalStringRegexpInfixExpression(operator string, left, right object.Object) object.Object {
	// defer untrace(trace("evalStringRegexpInfixExpression", operator, left, right))

	leftVal := left.(*object.String).Value
	rightVal := right.(*object.Regexp).Regexp
	matched, err := rightVal.MatchString(leftVal)
	if err != nil {
		return newError("regexp match failed: %s", err)
	}

	switch operator {
	case "=~":
		return nativeBoolToBooleanObject(matched)
	case "!~":
		return nativeBoolToBooleanObject(!matched)
	default:
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalNullRegexpInfixExpression(operator string, left, right object.Object) object.Object {
	switch operator {
	case "=~":
		return FALSE
	case "!~":
		return TRUE
	default:
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func arrayContains(arr *object.Array, obj object.Object) (bool, error) {
	// defer untrace(trace("arrayContains", arr, obj))

	if _, ok := obj.(*object.Null); ok {
		return false, nil
	}

	if regexpObj, ok := obj.(*object.Regexp); ok {
		for idx, el := range arr.Elements {
			stringObj, ok := el.(*object.String)
			if !ok {
				return false, fmt.Errorf("type mismatch at index %d in array: %s vs STRING",
					idx, el.Type())
			}
			matched, err := regexpObj.MatchString(stringObj.Value)
			if err != nil {
				return false, fmt.Errorf("regexp match failed: %s", err)
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}

	for idx, el := range arr.Elements {
		if el.Type() != obj.Type() {
			return false, fmt.Errorf("type mismatch at index %d in array: %s vs %s",
				idx, el.Type(), obj.Type())
		}
		if el.Equals(obj) {
			return true, nil
		}
	}

	return false, nil
}

func evalArrayInfixExpression(operator string, left, right object.Object) object.Object {
	// defer untrace(trace("evalStringArrayInfixExpression", operator, left, right))

	leftVal := left.(*object.Array)

	switch operator {
	case "==":
		return nativeBoolToBooleanObject(left.Equals(right))
	case "!=":
		return nativeBoolToBooleanObject(!left.Equals(right))
	case "includes":
		contains, err := arrayContains(leftVal, right)
		if err != nil {
			return newError("%s", err.Error())
		}
		return nativeBoolToBooleanObject(contains)
	default:
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalIdentifier(node *ast.Identifier, scope Scope) object.Object {
	// defer untrace(trace("evalIdentifier"))

	val, ok := resolveScopedName(node.Value, scope)
	if !ok {
		return newError("identifier not found: %s", node.Value)
	}

	return val
}

func resolveScopedName(name string, scope Scope) (object.Object, bool) {
	if val, ok := scope.Get(name); ok {
		return val, true
	}
	return nil, false
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func evalExpressions(exps []ast.Expression, scope Scope) []object.Object {
	// defer untrace(trace("evalExpressions", exps))

	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, scope)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}
