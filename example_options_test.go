package conditional_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/buildkite/conditional"
)

func ExampleWithFunction() {
	branch := "release/2026-06-07"
	startsWith := conditional.WithFunction("starts_with", conditional.Function{
		Args:   []conditional.ValueType{conditional.StringType, conditional.StringType},
		Return: conditional.BoolType,
		Eval: func(args []conditional.Value) (conditional.Value, error) {
			value, ok := args[0].AsString()
			if !ok {
				return conditional.NullValue(), errors.New("value must be a string")
			}
			prefix, ok := args[1].AsString()
			if !ok {
				return conditional.NullValue(), errors.New("prefix must be a string")
			}
			return conditional.BoolValue(strings.HasPrefix(value, prefix)), nil
		},
	})

	ok, err := conditional.Evaluate(
		`starts_with(build.branch, "release/")`,
		conditional.Context{
			EntryPoint: conditional.EntryPointBuildCondition,
			Build: conditional.Build{
				Branch: &branch,
			},
		},
		startsWith,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(ok)

	// Output:
	// true
}

func ExampleNewEvaluator() {
	branch := "release/2026-06-07"
	startsWith := conditional.WithFunction("starts_with", conditional.Function{
		Args:   []conditional.ValueType{conditional.StringType, conditional.StringType},
		Return: conditional.BoolType,
		Eval: func(args []conditional.Value) (conditional.Value, error) {
			value, ok := args[0].AsString()
			if !ok {
				return conditional.NullValue(), errors.New("value must be a string")
			}
			prefix, ok := args[1].AsString()
			if !ok {
				return conditional.NullValue(), errors.New("prefix must be a string")
			}
			return conditional.BoolValue(strings.HasPrefix(value, prefix)), nil
		},
	})

	evaluator, err := conditional.NewEvaluator(startsWith)
	if err != nil {
		panic(err)
	}

	ok, err := evaluator.Evaluate(
		`starts_with(build.branch, "release/")`,
		conditional.Context{
			EntryPoint: conditional.EntryPointBuildCondition,
			Build: conditional.Build{
				Branch: &branch,
			},
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(ok)

	// Output:
	// true
}
