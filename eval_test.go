package conditional

import "testing"

func TestConditionalEvaluationSemantics(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "true literal",
			source:     upstreamEvaluatorSpec,
			expression: `true`,
			want:       true,
		},
		{
			name:       "false literal",
			source:     upstreamEvaluatorSpec,
			expression: `false`,
			want:       false,
		},
		{
			name:       "integer equality",
			source:     upstreamEvaluatorSpec,
			expression: `1 == 1`,
			want:       true,
		},
		{
			name:       "integer inequality",
			source:     upstreamEvaluatorSpec,
			expression: `1 != 2`,
			want:       true,
		},
		{
			name:       "boolean null comparison",
			source:     upstreamEvaluatorSpec,
			expression: `true == null`,
			want:       false,
		},
		{
			name:       "null equality",
			source:     upstreamEvaluatorSpec,
			expression: `null == null`,
			want:       true,
		},
		{
			name:       "null inequality",
			source:     upstreamEvaluatorSpec,
			expression: `null != null`,
			want:       false,
		},
		{
			name:       "string equality",
			source:     upstreamEvaluatorSpec,
			expression: `env("branch") == "main"`,
			ctx:        Context{BuildEnv: map[string]string{"branch": "main"}},
			want:       true,
		},
		{
			name:       "string inequality",
			source:     upstreamEvaluatorSpec,
			expression: `env("branch") != "production"`,
			ctx:        Context{BuildEnv: map[string]string{"branch": "main"}},
			want:       true,
		},
		{
			name:       "array includes string",
			source:     upstreamEvaluatorSpec,
			expression: `["eggs", "ham", "coffee"] includes "ham"`,
			want:       true,
		},
		{
			name:       "array excludes string",
			source:     upstreamEvaluatorSpec,
			expression: `["eggs", "ham", "coffee"] includes "fruit"`,
			want:       false,
		},
		{
			name:       "logical and short-circuits false left side",
			source:     upstreamEvaluatorSpec,
			expression: `false && missing.value == "x"`,
			want:       false,
		},
		{
			name:       "logical or short-circuits true left side",
			source:     upstreamEvaluatorSpec,
			expression: `true || missing.value == "x"`,
			want:       true,
		},
		{
			name:       "nested precedence with and before or",
			source:     upstreamParserSpec,
			expression: `"a" == "d" || "a" == "b" && "a" == "a"`,
			want:       false,
		},
		{
			name:       "grouped precedence overrides and before or",
			source:     upstreamParserSpec,
			expression: `("a" == "d" || "a" == "b") && "a" == "a"`,
			want:       false,
		},
		{
			name:       "ternary evaluates alternative branch",
			source:     upstreamEvaluatorSpec,
			expression: `1 == 2 ? 3 == 4 : 5 == 5`,
			want:       true,
		},
		{
			name:       "ternary evaluates consequence branch",
			source:     upstreamEvaluatorSpec,
			expression: `1 == 1 ? 3 == 4 : 5 == 5`,
			want:       false,
		},
		{
			name:       "nested ternary is right associative",
			source:     upstreamEvaluatorSpec,
			expression: `false ? true : false ? true : true`,
			want:       true,
		},
		{
			name:       "missing variable fails closed",
			source:     upstreamBuildConditionSpec,
			expression: `missing.value == "x"`,
			wantError:  ErrorKindEvaluation,
		},
		{
			name:       "non boolean result fails closed",
			source:     upstreamBuildValidatorSpec,
			expression: `"not boolean"`,
			wantError:  ErrorKindResult,
		},
	}

	runEvaluateCases(t, tests)
}
