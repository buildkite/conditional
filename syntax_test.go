package conditional

import "testing"

func TestConditionalSyntaxReference(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "equality comparator",
			source:     docsConditionalsSource,
			expression: `12345 == 12345`,
			want:       true,
		},
		{
			name:       "inequality comparator",
			source:     docsConditionalsSource,
			expression: `"feature-branch" != "main"`,
			want:       true,
		},
		{
			name:       "regex match comparator",
			source:     docsConditionalsSource,
			expression: `"v1.0" =~ /^v[0-9]+\.0$/`,
			want:       true,
		},
		{
			name:       "regex not match comparator",
			source:     docsConditionalsSource,
			expression: `"build all" !~ /skip tests/`,
			want:       true,
		},
		{
			name:       "logical operators respect precedence",
			source:     docsConditionalsSource,
			expression: `true || false && false`,
			want:       true,
		},
		{
			name:       "array includes operator",
			source:     docsConditionalsSource,
			expression: `["main", "production"] includes "production"`,
			want:       true,
		},
		{
			name:       "single and double quoted strings compare equally",
			source:     docsConditionalsSource,
			expression: `'feature-branch' == "feature-branch"`,
			want:       true,
		},
		{
			name:       "literals include true false and null",
			source:     docsConditionalsSource,
			expression: `true == true && false == false && null == null`,
			want:       true,
		},
		{
			name:       "parentheses change logical grouping",
			source:     docsConditionalsSource,
			expression: `(true || false) && false`,
			want:       false,
		},
		{
			name:       "prefix bang negates booleans",
			source:     docsConditionalsSource,
			expression: `!false`,
			want:       true,
		},
		{
			name:       "comments are ignored",
			source:     docsConditionalsSource,
			expression: "// ignored\ntrue && true\n// also ignored",
			want:       true,
		},
		{
			name:       "inline comments terminate expression text",
			source:     upstreamParserSpec,
			expression: "true // ignores comments on the same line",
			want:       true,
		},
	}

	runEvaluateCases(t, tests)
}

func TestConditionalSyntaxErrors(t *testing.T) {
	tests := []validateCase{
		{
			name:       "parse error for malformed comparator",
			source:     upstreamParserSpec,
			expression: `nope != == one`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "parse error for unsupported regexp flag",
			source:     upstreamParserSpec,
			expression: `"main" =~ /main/x`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "validation rejects local-only contains operator",
			source:     upstreamParserSpec,
			expression: `["main"] @> build.branch`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("main")},
			},
			wantError: ErrorKindValidation,
		},
		{
			name:       "validation rejects env without an argument",
			source:     upstreamBuildConditionSpec,
			expression: `env() == ""`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects build env with too many arguments",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("FOO", "BAR") == null`,
			wantError:  ErrorKindValidation,
		},
	}

	runValidateCases(t, tests)
}

func TestConditionalSyntaxEvaluationErrors(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "evaluation rejects local-only contains operator",
			source:     upstreamParserSpec,
			expression: `["main"] @> build.branch`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("main")},
			},
			wantError: ErrorKindValidation,
		},
		{
			name:       "evaluation rejects literal unsupported Buildkite env",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("BUILDKITE_AGENT_ACCESS_TOKEN") == null`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
		},
	}

	runEvaluateCases(t, tests)
}
