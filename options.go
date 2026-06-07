package conditional

import (
	"fmt"
	"strings"

	"github.com/buildkite/conditional/internal/object"
	"github.com/buildkite/conditional/internal/regex"
)

// Option configures conditional validation and evaluation.
type Option func(*optionSet) error

type optionSet struct {
	functions map[string]Function
}

// Evaluator validates and evaluates conditionals with reusable options.
//
// The zero value is a Buildkite-parity evaluator with no caller-owned
// functions.
type Evaluator struct {
	options optionSet
}

// NewEvaluator returns an evaluator with reusable options.
func NewEvaluator(opts ...Option) (Evaluator, error) {
	options, err := applyOptions(opts)
	if err != nil {
		return Evaluator{}, err
	}
	return Evaluator{options: options}, nil
}

// Validate parses expression for the selected Buildkite context using the
// evaluator's options.
func (e Evaluator) Validate(expression string, ctx Context) error {
	entryPoint, err := normalizeEntryPoint(ctx.EntryPoint)
	if err != nil {
		return err
	}
	return validate(expression, ctx, entryPoint, e.options)
}

// Evaluate evaluates expression in the selected Buildkite context using the
// evaluator's options.
func (e Evaluator) Evaluate(expression string, ctx Context) (bool, error) {
	entryPoint, err := normalizeEntryPoint(ctx.EntryPoint)
	if err != nil {
		return false, err
	}
	return evaluateWithOptions(expression, ctx, entryPoint, e.options)
}

// Function defines an opt-in conditional function.
type Function struct {
	Args   []ValueType
	Return ValueType
	Eval   func(args []Value) (Value, error)
}

// ValueType describes a conditional value type.
type ValueType string

const (
	// StringType is the conditional string type.
	StringType ValueType = "string"
	// NumberType is the conditional integer type.
	NumberType ValueType = "number"
	// BoolType is the conditional boolean type.
	BoolType ValueType = "boolean"
	// NullType is the conditional null type.
	NullType ValueType = "null"
	// RegexpType is the conditional regular expression type.
	RegexpType ValueType = "regular expression"
	// StringArrayType is the conditional string array type.
	StringArrayType ValueType = "string array"
)

// String returns a human-readable value type description.
func (t ValueType) String() string {
	typ, err := t.internal()
	if err != nil {
		return "invalid"
	}
	return typ.describe()
}

func (t ValueType) internal() (valueType, error) {
	switch t {
	case StringType:
		return stringType(), nil
	case NumberType:
		return numberType(), nil
	case BoolType:
		return boolType(), nil
	case NullType:
		return valueType{kind: kindNull}, nil
	case RegexpType:
		return valueType{kind: kindRegexp}, nil
	case StringArrayType:
		return stringArrayType(), nil
	default:
		return valueType{kind: kindUnknown}, validationError("invalid function value type")
	}
}

// Value is a conditional runtime value.
//
// The zero value represents null.
type Value struct {
	obj object.Object
}

// StringValue returns a string value.
func StringValue(value string) Value {
	return Value{obj: &object.String{Value: value}}
}

// NumberValue returns an integer value.
func NumberValue(value int64) Value {
	return Value{obj: &object.Integer{Value: value}}
}

// BoolValue returns a boolean value.
func BoolValue(value bool) Value {
	return Value{obj: &object.Boolean{Value: value}}
}

// NullValue returns a null value.
func NullValue() Value {
	return Value{obj: &object.Null{}}
}

// RegexpValue returns a regular expression value.
func RegexpValue(pattern string, flags string) (Value, error) {
	compiled, err := regex.Compile(pattern, flags)
	if err != nil {
		return Value{}, err
	}
	return Value{obj: &object.Regexp{Regexp: compiled, Flags: flags}}, nil
}

// StringArrayValue returns a string array value.
func StringArrayValue(values []string) Value {
	elements := make([]object.Object, 0, len(values))
	for _, value := range values {
		elements = append(elements, &object.String{Value: value})
	}
	return Value{obj: &object.Array{Elements: elements}}
}

// Type returns the value's conditional type.
func (v Value) Type() ValueType {
	switch v.object().(type) {
	case *object.String:
		return StringType
	case *object.Integer:
		return NumberType
	case *object.Boolean:
		return BoolType
	case *object.Null:
		return NullType
	case *object.Regexp:
		return RegexpType
	case *object.Array:
		return StringArrayType
	default:
		return ValueType(kindUnknown)
	}
}

// IsNull reports whether the value is null.
func (v Value) IsNull() bool {
	_, ok := v.object().(*object.Null)
	return ok
}

// AsString returns the string value, if this value is a string.
func (v Value) AsString() (string, bool) {
	value, ok := v.object().(*object.String)
	if !ok {
		return "", false
	}
	return value.Value, true
}

// AsNumber returns the integer value, if this value is an integer.
func (v Value) AsNumber() (int64, bool) {
	value, ok := v.object().(*object.Integer)
	if !ok {
		return 0, false
	}
	return value.Value, true
}

// AsBool returns the boolean value, if this value is a boolean.
func (v Value) AsBool() (bool, bool) {
	value, ok := v.object().(*object.Boolean)
	if !ok {
		return false, false
	}
	return value.Value, true
}

// AsRegexp returns the regular expression pattern and flags, if this value is a
// regular expression.
func (v Value) AsRegexp() (pattern string, flags string, ok bool) {
	value, ok := v.object().(*object.Regexp)
	if !ok {
		return "", "", false
	}
	return value.Regexp.String(), value.Flags, true
}

// AsStringArray returns a copy of the array values, if this value is a string
// array.
func (v Value) AsStringArray() ([]string, bool) {
	value, ok := v.object().(*object.Array)
	if !ok {
		return nil, false
	}

	values := make([]string, 0, len(value.Elements))
	for _, element := range value.Elements {
		stringElement, ok := element.(*object.String)
		if !ok {
			return nil, false
		}
		values = append(values, stringElement.Value)
	}
	return values, true
}

// String returns a human-readable value representation.
func (v Value) String() string {
	return v.object().String()
}

func (v Value) object() object.Object {
	if v.obj == nil {
		return &object.Null{}
	}
	return v.obj
}

func valueFromObject(obj object.Object) Value {
	return Value{obj: obj}
}

// WithFunction registers an opt-in conditional function.
func WithFunction(name string, function Function) Option {
	return func(options *optionSet) error {
		if err := validateFunctionName(name); err != nil {
			return err
		}
		if reservedFunctionName(name) {
			return validationError("function `%s` uses a reserved Buildkite name", name)
		}
		if function.Eval == nil {
			return validationError("function `%s` requires an Eval callback", name)
		}
		if _, err := function.signature(); err != nil {
			return err
		}

		function.Args = append([]ValueType(nil), function.Args...)
		if options.functions == nil {
			options.functions = map[string]Function{}
		}
		if _, ok := options.functions[name]; ok {
			return validationError("function `%s` is already registered", name)
		}
		options.functions[name] = function
		return nil
	}
}

func applyOptions(opts []Option) (optionSet, error) {
	var options optionSet
	for _, opt := range opts {
		if opt == nil {
			return optionSet{}, validationError("nil conditional option")
		}
		if err := opt(&options); err != nil {
			return optionSet{}, err
		}
	}
	return options, nil
}

func (f Function) signature() (functionSignature, error) {
	args := make([]valueKind, 0, len(f.Args))
	for _, arg := range f.Args {
		typ, err := arg.internal()
		if err != nil {
			return functionSignature{}, err
		}
		args = append(args, typ.kind)
	}

	ret, err := f.Return.internal()
	if err != nil {
		return functionSignature{}, err
	}

	return functionSignature{args: args, ret: ret}, nil
}

func (f Function) objectFunction(name string) object.Function {
	return func(args []object.Object) object.Object {
		values := make([]Value, 0, len(args))
		for _, arg := range args {
			values = append(values, valueFromObject(arg))
		}

		result, err := f.Eval(values)
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		if !resultMatchesType(result, f.Return) {
			return &object.Error{
				Message: fmt.Sprintf(
					"function %s returned %s, want %s",
					name,
					result.Type(),
					f.Return,
				),
			}
		}
		return result.object()
	}
}

func resultMatchesType(value Value, typ ValueType) bool {
	if value.IsNull() {
		return true
	}
	return value.Type() == typ
}

func validateFunctionName(name string) error {
	if name == "" {
		return validationError("function name is required")
	}
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
		return validationError("invalid function name `%s`", name)
	}
	switch name {
	case "true", "false", "null", "includes":
		return validationError("invalid function name `%s`", name)
	}
	if !isFunctionNameStart(name[0]) {
		return validationError("invalid function name `%s`", name)
	}
	for i := 1; i < len(name); i++ {
		if !isFunctionNamePart(name[i]) {
			return validationError("invalid function name `%s`", name)
		}
	}
	return nil
}

func isFunctionNameStart(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isFunctionNamePart(ch byte) bool {
	return isFunctionNameStart(ch) || '0' <= ch && ch <= '9' || ch == '.'
}

func reservedFunctionName(name string) bool {
	root := name
	if idx := strings.IndexByte(name, '.'); idx >= 0 {
		root = name[:idx]
	}

	switch root {
	case "build", "env", "organization", "pipeline", "step":
		return true
	default:
		return false
	}
}
