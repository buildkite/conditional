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
		{
			name:       "lookbehind fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"ab" =~ /(?<=a)b/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "negative lookbehind fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"cb" =~ /(?<!a)b/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "atomic group fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"aa" =~ /(?>a*)a/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "zero or one possessive quantifier fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"a" =~ /a?+/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "zero or more possessive quantifier fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"aaa" =~ /a*+/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "one or more possessive quantifier fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"aaa" =~ /a++/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "angle named capture fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"group" =~ /(?<name>group)/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "single quote named capture fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"group" =~ /(?'name'group)/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "conditional regexp fails during parsing",
			source:     upstreamConditionalRegexpModel,
			expression: `"a" =~ /(?(1)a|b)/`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "escaped unsupported tokens remain literals",
			source:     upstreamConditionalRegexpModel,
			expression: `"(?<=a)" =~ /\(\?<=a\)/`,
		},
		{
			name:       "character class unsupported tokens remain literals",
			source:     upstreamConditionalRegexpModel,
			expression: `"?" =~ /[(?<=]/`,
		},
		{
			name:       "posix character class keeps outer class open",
			source:     upstreamConditionalRegexpModel,
			expression: `"?" =~ /[[:digit:](?<=]/`,
		},
		{
			name:       "regexp comment contents are ignored by feature validator",
			source:     upstreamConditionalRegexpModel,
			expression: `"ab" =~ /a(?# (?<= literal)b/`,
		},
	}

	runValidateCases(t, tests)
}
