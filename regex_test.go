package conditional

import "testing"

func TestConditionalRegexSemantics(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "branch starts with features slash",
			source:     docsConditionalsSource,
			expression: `build.branch =~ /^features\//`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("features/api")},
			},
			want: true,
		},
		{
			name:       "raw dollar works as end anchor",
			source:     upstreamEvaluatorSpec,
			expression: `build.branch =~ /\/release-123$/`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("feature/release-123")},
			},
			want: true,
		},
		{
			name:       "escaped dollar matches a literal dollar",
			source:     upstreamEvaluatorSpec,
			expression: `"fee$" =~ /fee\$/`,
			want:       true,
		},
		{
			name:       "escaped dollar does not act as anchor",
			source:     upstreamEvaluatorSpec,
			expression: `"fee" =~ /fee\$/`,
			want:       false,
		},
		{
			name:       "tag version expression",
			source:     docsConditionalsSource,
			expression: `build.tag =~ /^v[0-9]+\.0$/`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Tag: str("v1.0")},
			},
			want: true,
		},
		{
			name:       "build env tag version expression",
			source:     docsConditionalsSource,
			expression: `build.env("BUILDKITE_TAG") =~ /^v[0-9]+\.0$/`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Tag: str("v2.0")},
			},
			want: true,
		},
		{
			name:       "case insensitive message exclusion",
			source:     docsConditionalsSource,
			expression: `build.message !~ /\[skip tests\]/i`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Message: str("run all tests")},
			},
			want: true,
		},
		{
			name:       "case insensitive message match",
			source:     upstreamEvaluatorSpec,
			expression: `build.message =~ /\[skip tests\]/i`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Message: str("[SKIP TESTS] please")},
			},
			want: true,
		},
		{
			name:       "posix digit class",
			source:     upstreamEvaluatorSpec,
			expression: `"v123" =~ /^v[[:digit:]]+$/`,
			want:       true,
		},
	}

	runEvaluateCases(t, tests)
}

func TestConditionalRegexValidation(t *testing.T) {
	tests := []validateCase{
		{
			name:       "unsupported flags fail during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"main" =~ /main/x`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "unterminated regex fails during parsing",
			source:     upstreamParserSpec,
			expression: `"main" =~ /main`,
			wantError:  ErrorKindParse,
		},
	}

	runValidateCases(t, tests)
}
