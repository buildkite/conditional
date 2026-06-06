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
			name:       "array includes regex",
			source:     upstreamEvaluatorSpec,
			expression: `build.creator.teams includes /dep/`,
			ctx: Context{
				Build: Build{
					Creator: Actor{Teams: []string{"deploy", "platform"}},
				},
			},
			want: true,
		},
		{
			name:       "array includes enum string variable",
			source:     docsConditionalsSource,
			expression: `["passed", "failed"] includes build.state`,
			ctx: Context{
				Build: Build{State: str("passed")},
			},
			want: true,
		},
		{
			name:       "documented started failing build state",
			source:     docsConditionalsSource,
			expression: `build.state == "started_failing"`,
			ctx: Context{
				Build: Build{State: str("started_failing")},
			},
			want: true,
		},
		{
			name:       "enum comparison allows interpolated string literal",
			source:     upstreamParserSpec,
			expression: `build.state == "${STATE}"`,
			ctx: Context{
				Build:    Build{State: str("passed")},
				BuildEnv: map[string]string{"STATE": "passed"},
			},
			want: true,
		},
		{
			name:       "valid enum comparison can put literal first",
			source:     upstreamParserSpec,
			expression: `"passed" == build.state`,
			ctx: Context{
				Build: Build{State: str("passed")},
			},
			want: true,
		},
		{
			name:       "null includes string evaluates false",
			source:     upstreamEvaluatorSpec,
			expression: `null includes "fruit"`,
			want:       false,
		},
		{
			name:       "null regex match evaluates false",
			source:     upstreamEvaluatorSpec,
			expression: `null =~ /main|development/`,
			want:       false,
		},
		{
			name:       "null regex non match evaluates true",
			source:     upstreamEvaluatorSpec,
			expression: `null !~ /main|development/`,
			want:       true,
		},
		{
			name:       "logical and short-circuits failing shell expansion",
			source:     upstreamEvaluatorSpec,
			expression: `false && ${notset:?} == "x"`,
			want:       false,
		},
		{
			name:       "logical or short-circuits failing shell expansion",
			source:     upstreamEvaluatorSpec,
			expression: `true || ${notset:?} == "x"`,
			want:       true,
		},
		{
			name:       "logical and short-circuits missing variable validation",
			source:     upstreamEvaluatorSpec,
			expression: `false && missing.value == "x"`,
			want:       false,
		},
		{
			name:       "logical or short-circuits missing variable validation",
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
			name:       "ternary array branch can feed includes",
			source:     upstreamEvaluatorSpec,
			expression: `(env("USE_CREATOR") == "true" ? build.creator.teams : ["deploy"]) includes "deploy"`,
			ctx:        Context{BuildEnv: map[string]string{"USE_CREATOR": "false"}},
			want:       true,
		},
		{
			name:       "missing variable fails closed",
			source:     upstreamBuildConditionSpec,
			expression: `missing.value == "x"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "non boolean result fails closed",
			source:     upstreamBuildValidatorSpec,
			expression: `"not boolean"`,
			wantError:  ErrorKindResult,
		},
		{
			name:       "nullable ternary result fails closed",
			source:     upstreamParserSpec,
			expression: `true ? null : true`,
			wantError:  ErrorKindResult,
		},
		{
			name:       "nullable ternary alternative result fails closed",
			source:     upstreamParserSpec,
			expression: `false ? true : null`,
			wantError:  ErrorKindResult,
		},
		{
			name:       "nullable pull request boolean can be negated",
			source:     docsConditionalsSource,
			expression: `!build.pull_request.draft`,
			want:       true,
		},
		{
			name:       "nullable pull request boolean can be guarded before logical evaluation",
			source:     docsConditionalsSource,
			expression: `build.pull_request.draft != null && build.pull_request.draft`,
			want:       false,
		},
		{
			name:       "nullable pull request boolean fails before logical evaluation",
			source:     docsConditionalsSource,
			expression: `build.pull_request.draft || false`,
			wantError:  ErrorKindValidation,
		},
	}

	runEvaluateCases(t, tests)
}

func TestConditionalShellSubstitutionEvaluation(t *testing.T) {
	ctx := Context{
		BuildEnv: map[string]string{
			"branch": "main",
			"empty":  "",
			"tag":    "foo",
			"two":    "2",
		},
	}

	tests := []evaluateCase{
		{
			name:       "bare shell variable",
			source:     upstreamEvaluatorSpec,
			expression: `$branch == "main"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "bare shell variable preserves dotted suffix",
			source:     upstreamEvaluatorSpec,
			expression: `$branch.deploy == "main.deploy"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "braced shell variable",
			source:     upstreamEvaluatorSpec,
			expression: `${branch} == "main"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "unset shell variable is null",
			source:     upstreamEvaluatorSpec,
			expression: `${notset} == null`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "required unset shell variable fails",
			source:     upstreamEvaluatorSpec,
			expression: `${notset:?} == "error"`,
			ctx:        ctx,
			wantError:  ErrorKindEvaluation,
		},
		{
			name:       "dash default uses set value",
			source:     upstreamEvaluatorSpec,
			expression: `${branch-xx} == "main"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "dash default uses fallback for unset",
			source:     upstreamEvaluatorSpec,
			expression: `${notset-xx} == "xx"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "plus alternate uses alternate for set",
			source:     upstreamEvaluatorSpec,
			expression: `${branch+xx} == "xx"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "plus alternate returns empty for unset",
			source:     upstreamEvaluatorSpec,
			expression: `${notset+xx} == ""`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "colon dash treats empty as unset",
			source:     upstreamEvaluatorSpec,
			expression: `${empty:-xx} == "xx"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "colon plus treats empty as unset",
			source:     upstreamEvaluatorSpec,
			expression: `${empty:+xx} == ""`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "substring literal offsets",
			source:     upstreamEvaluatorSpec,
			expression: `${branch:1:2} == "ai"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "substring variable length",
			source:     upstreamEvaluatorSpec,
			expression: `${branch:2:$two} == "in"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "substring nested expressions",
			source:     upstreamEvaluatorSpec,
			expression: `${branch:${empty:-1}:${two+2}} == "ai"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "substring past end returns empty",
			source:     upstreamEvaluatorSpec,
			expression: `${branch:25:2} == ""`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "substring rejects non integer length",
			source:     upstreamEvaluatorSpec,
			expression: `${branch:3:$tag} == "error"`,
			ctx:        ctx,
			wantError:  ErrorKindEvaluation,
		},
		{
			name:       "double quoted strings interpolate shell variables",
			source:     upstreamEvaluatorSpec,
			expression: `"${branch}" == "main"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "double quoted interpolation preserves backslashes",
			source:     upstreamParserSpec,
			expression: `"C:\\${branch}" == "C:\\main"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "double dollar becomes literal dollar in double quoted strings",
			source:     upstreamParserSpec,
			expression: `"cost $$5" == "cost $5"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "single quoted strings do not interpolate shell variables",
			source:     upstreamEvaluatorSpec,
			expression: `'${branch}' == "main"`,
			ctx:        ctx,
			want:       false,
		},
	}

	runEvaluateCases(t, tests)
}
