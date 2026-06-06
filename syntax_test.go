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
			name:       "parse error for malformed dotted identifier",
			source:     upstreamParserSpec,
			expression: `build..branch == "main"`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "parse error for out of range octal string escape",
			source:     upstreamConditionalGrammar,
			expression: `"\400" == ""`,
			wantError:  ErrorKindParse,
		},
		{
			name:       "parser rejects local-only contains operator",
			source:     upstreamParserSpec,
			expression: `["main"] @> build.branch`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("main")},
			},
			wantError: ErrorKindParse,
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
		{
			name:       "validation rejects env non string argument",
			source:     upstreamParserSpec,
			expression: `env(1) == ""`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects env escaped dollar name",
			source:     upstreamBuildConditionSpec,
			expression: `env("$$FOO") == ""`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects env backslash escaped dollar name",
			source:     upstreamBuildConditionSpec,
			expression: `env("\$FOO") == ""`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects unknown variable",
			source:     upstreamParserSpec,
			expression: `not_a_variable == "x"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects unknown function",
			source:     upstreamParserSpec,
			expression: `not_a_function() == "x"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects regex with non regexp rhs",
			source:     upstreamParserSpec,
			expression: `"d" =~ "hello"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects regex with boolean lhs",
			source:     upstreamParserSpec,
			expression: `true =~ /true/`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects mismatched equality types",
			source:     upstreamParserSpec,
			expression: `1 == "one"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects boolean string comparison",
			source:     upstreamParserSpec,
			expression: `true == "true"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects includes on non array",
			source:     upstreamParserSpec,
			expression: `"not-an-array" includes "one"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects invalid enum literal",
			source:     upstreamBuildConditionSpec,
			expression: `build.state == "faillled"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects escaped static dollar enum literal",
			source:     upstreamConditionalGrammar,
			expression: `build.state == "\\\$STATE"`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects reversed enum comparison",
			source:     upstreamParserSpec,
			expression: `"faillled" == build.state`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects enum in string regex position",
			source:     upstreamParserSpec,
			expression: `build.state =~ /pass/`,
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validation rejects ternary branch type mismatch",
			source:     upstreamParserSpec,
			expression: `(true ? "string" : 123) == null`,
			wantError:  ErrorKindValidation,
		},
	}

	runValidateCases(t, tests)
}

func TestConditionalSyntaxEvaluationErrors(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "evaluation rejects local-only contains operator at parse time",
			source:     upstreamParserSpec,
			expression: `["main"] @> build.branch`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("main")},
			},
			wantError: ErrorKindParse,
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
