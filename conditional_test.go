package conditional

import (
	"errors"
	"strings"
	"testing"
)

func TestRootValidateAndEvaluateErrorKinds(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "parse error",
			source:     upstreamBuildConditionSpec,
			expression: `nope != == one`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindParse,
		},
		{
			name:       "evaluation error",
			source:     upstreamBuildConditionSpec,
			expression: `${notset:?} == "x"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindEvaluation,
		},
		{
			name:       "result error",
			source:     upstreamBuildValidatorSpec,
			expression: `"not boolean"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindResult,
		},
		{
			name:       "validation error",
			source:     upstreamBuildValidatorSpec,
			expression: `step.key == "deploy"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
		},
	}

	runEvaluateCases(t, tests)
}

func TestRootValidateErrorKinds(t *testing.T) {
	tests := []validateCase{
		{
			name:       "parse error",
			source:     upstreamBuildConditionSpec,
			expression: `nope != == one`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindParse,
		},
		{
			name:       "result error",
			source:     upstreamBuildValidatorSpec,
			expression: `"not boolean"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindResult,
		},
		{
			name:       "validation error",
			source:     upstreamBuildValidatorSpec,
			expression: `step.key == "deploy"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
		},
	}

	runValidateCases(t, tests)
}

func TestParseErrorUnwrapsParserErrors(t *testing.T) {
	err := Validate(`nope != == one`, Context{EntryPoint: EntryPointBuildCondition})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !IsErrorKind(err, ErrorKindParse) {
		t.Fatalf("expected parse error kind, got %v", err)
	}

	cause := errors.Unwrap(err)
	if cause == nil {
		t.Fatal("expected parse error to unwrap to parser errors")
	}
	if !strings.Contains(cause.Error(), "no prefix parse function for == found") {
		t.Fatalf("unexpected parse cause: %v", cause)
	}
}
