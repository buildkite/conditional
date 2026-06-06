package conditional

import "testing"

func TestConformanceEvaluateCases(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "docs branch is main or production",
			source:     docsConditionalsSource,
			expression: `build.branch == "main" || build.branch == "production"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("main")},
			},
			want: true,
		},
		{
			name:       "docs feature branch regex",
			source:     docsConditionalsSource,
			expression: `build.branch =~ /^features\//`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("features/api")},
			},
			want: true,
		},
		{
			name:       "docs tag regex via build env",
			source:     docsConditionalsSource,
			expression: `build.env("BUILDKITE_TAG") =~ /^v[0-9]+\.0$/`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Tag: str("v2.0")},
			},
			want: true,
		},
		{
			name:       "docs custom build env",
			source:     docsConditionalsSource,
			expression: `build.env("CUSTOM_ENVIRONMENT_VARIABLE") == "value"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv:   map[string]string{"CUSTOM_ENVIRONMENT_VARIABLE": "value"},
			},
			want: true,
		},
		{
			name:       "docs creator teams includes deploy",
			source:     docsConditionalsSource,
			expression: `build.creator.teams includes "deploy"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					Creator: Actor{Teams: []string{"deploy", "platform"}},
				},
			},
			want: true,
		},
		{
			name:       "docs merge queue base branch",
			source:     docsConditionalsSource,
			expression: `build.merge_queue.base_branch == "main"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					MergeQueue: MergeQueue{BaseBranch: str("main")},
				},
			},
			want: true,
		},
		{
			name:       "upstream null regex match is false",
			source:     upstreamEvaluatorSpec,
			expression: `null =~ /main|development/`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			want:       false,
		},
		{
			name:       "upstream shell substitution default",
			source:     upstreamEvaluatorSpec,
			expression: `${notset:-fallback} == "fallback"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			want:       true,
		},
		{
			name:       "upstream ternary alternative branch",
			source:     upstreamEvaluatorSpec,
			expression: `1 == 2 ? 3 == 4 : 5 == 5`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			want:       true,
		},
		{
			name:       "build notification parse errors are false",
			source:     upstreamBuildNotificationSpec,
			expression: `nope != == one`,
			ctx:        Context{EntryPoint: EntryPointBuildNotification},
			want:       false,
		},
	}

	runEvaluateCases(t, tests)
}

func TestConformanceValidateCases(t *testing.T) {
	tests := []validateCase{
		{
			name:       "validate rejects unsupported Buildkite env",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("BUILDKITE_AGENT_ACCESS_TOKEN") == null`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
		},
		{
			name:       "validate rejects step variables without step entrypoint",
			source:     upstreamBuildValidatorSpec,
			expression: `step.outcome == "hard_failed"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
		},
	}

	runValidateCases(t, tests)
}
