package conditional

import "testing"

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
