package conditional

import (
	"errors"
	"strings"
	"testing"
)

func TestCustomFunctionOption(t *testing.T) {
	startsWith := WithFunction("starts_with", Function{
		Args:   []ValueType{StringType, StringType},
		Return: BoolType,
		Eval: func(args []Value) (Value, error) {
			value, ok := args[0].AsString()
			if !ok {
				return NullValue(), errors.New("value must be a string")
			}
			prefix, ok := args[1].AsString()
			if !ok {
				return NullValue(), errors.New("prefix must be a string")
			}
			return BoolValue(strings.HasPrefix(value, prefix)), nil
		},
	})

	ctx := Context{Build: Build{Branch: str("main")}}
	expression := `starts_with(build.branch, "ma")`

	if err := Validate(expression, ctx, startsWith); err != nil {
		t.Fatalf("Validate(%q) returned error: %v", expression, err)
	}

	got, err := Evaluate(expression, ctx, startsWith)
	if err != nil {
		t.Fatalf("Evaluate(%q) returned error: %v", expression, err)
	}
	if !got {
		t.Fatalf("Evaluate(%q) = false, want true", expression)
	}
}

func TestEvaluatorUsesReusableOptions(t *testing.T) {
	startsWith := WithFunction("starts_with", Function{
		Args:   []ValueType{StringType, StringType},
		Return: BoolType,
		Eval: func(args []Value) (Value, error) {
			value, ok := args[0].AsString()
			if !ok {
				return NullValue(), errors.New("value must be a string")
			}
			prefix, ok := args[1].AsString()
			if !ok {
				return NullValue(), errors.New("prefix must be a string")
			}
			return BoolValue(strings.HasPrefix(value, prefix)), nil
		},
	})

	evaluator, err := NewEvaluator(startsWith)
	if err != nil {
		t.Fatalf("NewEvaluator returned error: %v", err)
	}

	ctx := Context{Build: Build{Branch: str("main")}}
	expression := `starts_with(build.branch, "ma")`

	if err := evaluator.Validate(expression, ctx); err != nil {
		t.Fatalf("Evaluator.Validate(%q) returned error: %v", expression, err)
	}

	got, err := evaluator.Evaluate(expression, ctx)
	if err != nil {
		t.Fatalf("Evaluator.Evaluate(%q) returned error: %v", expression, err)
	}
	if !got {
		t.Fatalf("Evaluator.Evaluate(%q) = false, want true", expression)
	}
}

func TestEvaluatorZeroValueRejectsUnknownFunction(t *testing.T) {
	var evaluator Evaluator

	err := evaluator.Validate(`starts_with("main", "ma")`, Context{})
	if !IsErrorKind(err, ErrorKindValidation) {
		t.Fatalf("Evaluator.Validate error = %v, want %s", err, ErrorKindValidation)
	}
}

func TestNewEvaluatorValidatesOptions(t *testing.T) {
	_, err := NewEvaluator(WithFunction("env", Function{
		Args:   []ValueType{StringType},
		Return: StringType,
		Eval: func(args []Value) (Value, error) {
			return StringValue("x"), nil
		},
	}))
	if !IsErrorKind(err, ErrorKindValidation) {
		t.Fatalf("NewEvaluator error = %v, want %s", err, ErrorKindValidation)
	}
}

func TestCustomFunctionReturnsComparableValue(t *testing.T) {
	lower := WithFunction("lower", Function{
		Args:   []ValueType{StringType},
		Return: StringType,
		Eval: func(args []Value) (Value, error) {
			value, ok := args[0].AsString()
			if !ok {
				return NullValue(), errors.New("argument must be a string")
			}
			return StringValue(strings.ToLower(value)), nil
		},
	})

	got, err := Evaluate(`lower("MAIN") == "main"`, Context{}, lower)
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if !got {
		t.Fatal("Evaluate = false, want true")
	}
}

func TestUnknownFunctionStillFailsWithoutOption(t *testing.T) {
	err := Validate(`starts_with("main", "ma")`, Context{})
	if !IsErrorKind(err, ErrorKindValidation) {
		t.Fatalf("Validate error = %v, want %s", err, ErrorKindValidation)
	}
}

func TestCustomFunctionArgumentValidation(t *testing.T) {
	startsWith := WithFunction("starts_with", Function{
		Args:   []ValueType{StringType, StringType},
		Return: BoolType,
		Eval: func(args []Value) (Value, error) {
			return BoolValue(true), nil
		},
	})

	err := Validate(`starts_with(1, "ma")`, Context{}, startsWith)
	if !IsErrorKind(err, ErrorKindValidation) {
		t.Fatalf("Validate error = %v, want %s", err, ErrorKindValidation)
	}
}

func TestCustomFunctionEvaluationError(t *testing.T) {
	explode := WithFunction("explode", Function{
		Return: BoolType,
		Eval: func(args []Value) (Value, error) {
			return NullValue(), errors.New("boom")
		},
	})

	_, err := Evaluate(`explode()`, Context{}, explode)
	if !IsErrorKind(err, ErrorKindEvaluation) {
		t.Fatalf("Evaluate error = %v, want %s", err, ErrorKindEvaluation)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Evaluate error = %v, want message containing boom", err)
	}
}

func TestCustomFunctionEvaluationErrorInNotificationFailsClosed(t *testing.T) {
	explode := WithFunction("explode", Function{
		Return: BoolType,
		Eval: func(args []Value) (Value, error) {
			return NullValue(), errors.New("boom")
		},
	})

	got, err := Evaluate(`explode()`, Context{EntryPoint: EntryPointBuildNotification}, explode)
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if got {
		t.Fatal("Evaluate = true, want false")
	}
}

func TestCustomFunctionOptionValidation(t *testing.T) {
	tests := []struct {
		name   string
		option Option
	}{
		{
			name: "reserved built in function",
			option: WithFunction("env", Function{
				Args:   []ValueType{StringType},
				Return: StringType,
				Eval: func(args []Value) (Value, error) {
					return StringValue("x"), nil
				},
			}),
		},
		{
			name: "reserved build namespace",
			option: WithFunction("build.custom", Function{
				Return: BoolType,
				Eval: func(args []Value) (Value, error) {
					return BoolValue(true), nil
				},
			}),
		},
		{
			name: "invalid function name",
			option: WithFunction(".custom", Function{
				Return: BoolType,
				Eval: func(args []Value) (Value, error) {
					return BoolValue(true), nil
				},
			}),
		},
		{
			name: "missing eval callback",
			option: WithFunction("custom", Function{
				Return: BoolType,
			}),
		},
		{
			name: "missing return type",
			option: WithFunction("custom", Function{
				Eval: func(args []Value) (Value, error) {
					return BoolValue(true), nil
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(`true`, Context{}, tt.option)
			if !IsErrorKind(err, ErrorKindValidation) {
				t.Fatalf("Validate error = %v, want %s", err, ErrorKindValidation)
			}
		})
	}
}

func TestCustomFunctionReturnTypeValidation(t *testing.T) {
	badReturn := WithFunction("bad_return", Function{
		Return: BoolType,
		Eval: func(args []Value) (Value, error) {
			return StringValue("not a bool"), nil
		},
	})

	_, err := Evaluate(`bad_return()`, Context{}, badReturn)
	if !IsErrorKind(err, ErrorKindEvaluation) {
		t.Fatalf("Evaluate error = %v, want %s", err, ErrorKindEvaluation)
	}
	if !strings.Contains(err.Error(), "returned string, want boolean") {
		t.Fatalf("Evaluate error = %v, want return type message", err)
	}
}
