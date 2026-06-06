package conditional

import "testing"

func TestBuildkiteDocsExamples(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "branch is main or production",
			source:     docsConditionalsSource,
			expression: `build.branch == "main" || build.branch == "production"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("main")},
			},
			want: true,
		},
		{
			name:       "branch is not production",
			source:     docsConditionalsSource,
			expression: `build.branch != "production"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("main")},
			},
			want: true,
		},
		{
			name:       "building a tag",
			source:     docsConditionalsSource,
			expression: `build.tag != null`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Tag: str("v1.2.3")},
			},
			want: true,
		},
		{
			name:       "build was created from schedule",
			source:     docsConditionalsSource,
			expression: `build.source == "schedule"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Source: str("schedule")},
			},
			want: true,
		},
		{
			name:       "custom build env matches",
			source:     docsConditionalsSource,
			expression: `build.env("CUSTOM_ENVIRONMENT_VARIABLE") == "value"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv:   map[string]string{"CUSTOM_ENVIRONMENT_VARIABLE": "value"},
			},
			want: true,
		},
		{
			name:       "creator teams includes deploy",
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
			name:       "non draft pull request",
			source:     docsConditionalsSource,
			expression: `!build.pull_request.draft`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					PullRequest: PullRequest{Draft: boolptr(false)},
				},
			},
			want: true,
		},
		{
			name:       "merge queue targets main",
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
	}

	runEvaluateCases(t, tests)
}

func TestBuildkiteContextAssignments(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "organization slug",
			source:     upstreamBuildConditionSpec,
			expression: `organization.slug == "org-slug-town"`,
			ctx: Context{
				EntryPoint:   EntryPointBuildCondition,
				Organization: Organization{Slug: str("org-slug-town")},
			},
			want: true,
		},
		{
			name:       "pipeline slug",
			source:     upstreamBuildConditionSpec,
			expression: `pipeline.slug == "slug-town"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Pipeline:   Pipeline{Slug: str("slug-town")},
			},
			want: true,
		},
		{
			name:       "build source event from webhook env",
			source:     upstreamBuildConditionSpec,
			expression: `build.source_event == "pull_request"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Source: str("webhook")},
				BuildEnv:   map[string]string{"BUILDKITE_GITHUB_EVENT": "pull_request"},
			},
			want: true,
		},
		{
			name:       "build source event null for non webhook",
			source:     upstreamBuildConditionSpec,
			expression: `build.source_event == null`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Source: str("api")},
				BuildEnv:   map[string]string{"BUILDKITE_GITHUB_EVENT": "pull_request"},
			},
			want: true,
		},
		{
			name:       "build source action from webhook env",
			source:     upstreamBuildConditionSpec,
			expression: `build.source_action == "labeled"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Source: str("webhook")},
				BuildEnv:   map[string]string{"BUILDKITE_GITHUB_ACTION": "labeled"},
			},
			want: true,
		},
		{
			name:       "pull request label only for labeled event",
			source:     upstreamBuildConditionSpec,
			expression: `build.pull_request.label == "test-gpu"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					Source:       str("webhook"),
					SourceEvent:  str("pull_request"),
					SourceAction: str("labeled"),
					PullRequest:  PullRequest{Label: str("test-gpu")},
				},
			},
			want: true,
		},
		{
			name:       "pull request label null for opened event",
			source:     upstreamBuildConditionSpec,
			expression: `build.pull_request.label == null`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					Source:       str("webhook"),
					SourceEvent:  str("pull_request"),
					SourceAction: str("opened"),
					PullRequest:  PullRequest{Label: str("test-gpu")},
				},
			},
			want: true,
		},
		{
			name:       "pull request repository and fork",
			source:     docsConditionalsSource,
			expression: `build.pull_request.repository == "git@github.com:acme/repo.git" && build.pull_request.repository.fork`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					PullRequest: PullRequest{
						Repository:     str("git@github.com:acme/repo.git"),
						RepositoryFork: boolptr(true),
					},
				},
			},
			want: true,
		},
		{
			name:       "build fixed",
			source:     upstreamBuildConditionSpec,
			expression: `build.fixed`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Fixed: boolptr(true)},
			},
			want: true,
		},
		{
			name:       "pipeline started passing",
			source:     upstreamBuildConditionSpec,
			expression: `pipeline.started_passing && !pipeline.started_failing`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Pipeline: Pipeline{
					StartedPassing: boolptr(true),
					StartedFailing: boolptr(false),
				},
			},
			want: true,
		},
	}

	runEvaluateCases(t, tests)
}

func TestBuildkiteEnvironmentFunctions(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "env sees build env override",
			source:     upstreamBuildConditionSpec,
			expression: `env("DEPLOY_ENV") == "build"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				ProjectEnv: map[string]string{
					"DEPLOY_ENV": "project",
				},
				BuildEnv: map[string]string{
					"DEPLOY_ENV": "build",
				},
			},
			want: true,
		},
		{
			name:       "env returns empty string for missing values",
			source:     upstreamBuildNotificationSpec,
			expression: `env("MISSING") == ""`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			want:       true,
		},
		{
			name:       "build env returns null for missing values",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("MISSING") == null`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			want:       true,
		},
		{
			name:       "build env reads project env",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("PROJECTED") == "fully"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				ProjectEnv: map[string]string{
					"PROJECTED": "fully",
				},
			},
			want: true,
		},
		{
			name:       "build env preserves present empty string",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("EMPTY") == ""`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv:   map[string]string{"EMPTY": ""},
			},
			want: true,
		},
		{
			name:       "built in tag becomes empty string for env",
			source:     upstreamBuildConditionSpec,
			expression: `env("BUILDKITE_TAG") == ""`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			want:       true,
		},
		{
			name:       "built in pull request defaults false",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("BUILDKITE_PULL_REQUEST") == "false"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			want:       true,
		},
		{
			name:       "supported conditional only Buildkite env is exposed",
			source:     upstreamBuildConditionSpec,
			expression: `env("BUILDKITE_GITHUB_REVIEW_STATE") == "approved" && build.env("BUILDKITE_GITHUB_REVIEW_STATE") == "approved"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv:   map[string]string{"BUILDKITE_GITHUB_REVIEW_STATE": "approved"},
			},
			want: true,
		},
		{
			name:   "built in Buildkite env values are derived from context",
			source: upstreamBuildConditionSpec,
			expression: `build.env("BUILDKITE_BRANCH") == "main" &&
				build.env("BUILDKITE_TAG") == "v1.2.3" &&
				env("BUILDKITE_MESSAGE") == "ship it" &&
				build.env("BUILDKITE_COMMIT") == "abc123" &&
				build.env("BUILDKITE_PIPELINE_NAME") == "Deploy" &&
				build.env("BUILDKITE_PIPELINE_SLUG") == "deploy" &&
				build.env("BUILDKITE_PIPELINE_ID") == "018f" &&
				build.env("BUILDKITE_REPO") == "git@github.com:acme/repo.git" &&
				build.env("BUILDKITE_ORGANIZATION_SLUG") == "acme" &&
				build.env("BUILDKITE_PULL_REQUEST") == "123" &&
				build.env("BUILDKITE_PULL_REQUEST_BASE_BRANCH") == "main" &&
				build.env("BUILDKITE_PULL_REQUEST_REPO") == "git@github.com:acme/repo.git" &&
				build.env("BUILDKITE_PULL_REQUEST_LABELS") == "bug,deploy" &&
				build.env("BUILDKITE_MERGE_QUEUE_BASE_BRANCH") == "main" &&
				build.env("BUILDKITE_MERGE_QUEUE_BASE_COMMIT") == "def456"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					Branch:  str("main"),
					Tag:     str("v1.2.3"),
					Message: str("ship it"),
					Commit:  str("abc123"),
					PullRequest: PullRequest{
						ID:         str("123"),
						BaseBranch: str("main"),
						Repository: str("git@github.com:acme/repo.git"),
						Labels:     []string{"bug", "deploy"},
					},
					MergeQueue: MergeQueue{
						BaseBranch: str("main"),
						BaseCommit: str("def456"),
					},
				},
				Pipeline: Pipeline{
					Name:       str("Deploy"),
					Slug:       str("deploy"),
					ID:         str("018f"),
					Repository: str("git@github.com:acme/repo.git"),
				},
				Organization: Organization{Slug: str("acme")},
			},
			want: true,
		},
		{
			name:       "unsupported Buildkite env is filtered when indirect",
			source:     upstreamBuildConditionSpec,
			expression: `build.env(env("SECRET_NAME")) == null && env(env("SECRET_NAME")) == ""`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv: map[string]string{
					"SECRET_NAME":                  "BUILDKITE_AGENT_ACCESS_TOKEN",
					"BUILDKITE_AGENT_ACCESS_TOKEN": "secret",
				},
			},
			want: true,
		},
	}

	runEvaluateCases(t, tests)
}

func TestBuildkiteContextValidation(t *testing.T) {
	tests := []validateCase{
		{
			name:       "blank condition is valid",
			source:     upstreamBuildValidatorSpec,
			expression: "   ",
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
		},
		{
			name:       "invalid conditional is rejected",
			source:     upstreamBuildValidatorSpec,
			expression: "lol",
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindEvaluation,
		},
		{
			name:       "step variables rejected without step option",
			source:     upstreamBuildValidatorSpec,
			expression: `step.outcome == "hard_failed"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
		},
		{
			name:       "step variables accepted with step option",
			source:     upstreamBuildValidatorSpec,
			expression: `step.outcome == "hard_failed"`,
			ctx:        Context{EntryPoint: EntryPointBuildConditionWithStep},
		},
		{
			name:       "literal unsupported Buildkite env is rejected",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("BUILDKITE_AGENT_ACCESS_TOKEN") == null`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
		},
	}

	runValidateCases(t, tests)
}

func TestBuildkiteEntrypoints(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "evaluate defaults to build condition",
			source:     upstreamBuildConditionSpec,
			expression: `build.branch == "features/api"`,
			ctx: Context{
				Build: Build{Branch: str("features/api")},
			},
			want: true,
		},
		{
			name:       "unknown entrypoint rejects evaluation",
			source:     upstreamBuildConditionSpec,
			expression: `true`,
			ctx:        Context{EntryPoint: "unknown"},
			wantError:  ErrorKindValidation,
		},
		{
			name:       "build notification blank condition is deliverable",
			source:     upstreamBuildNotificationSpec,
			expression: ` `,
			ctx:        Context{EntryPoint: EntryPointBuildNotification},
			want:       true,
		},
		{
			name:       "build notification false when condition fails",
			source:     upstreamBuildNotificationSpec,
			expression: `env("BUILDKITE_BRANCH") == "not-the-one"`,
			ctx: Context{
				EntryPoint: EntryPointBuildNotification,
				Build:      Build{Branch: str("main")},
			},
			want: false,
		},
		{
			name:       "build notification false when unparseable",
			source:     upstreamBuildNotificationSpec,
			expression: `nope != == one`,
			ctx:        Context{EntryPoint: EntryPointBuildNotification},
			want:       false,
		},
		{
			name:       "build notification false when step variable unavailable",
			source:     upstreamBuildNotificationSpec,
			expression: `step.key == "deploy"`,
			ctx:        Context{EntryPoint: EntryPointBuildNotification},
			want:       false,
		},
		{
			name:       "step notification can use step variables",
			source:     upstreamStepNotificationSpec,
			expression: `step.key == "foo"`,
			ctx: Context{
				EntryPoint: EntryPointStepNotification,
				Step:       &Step{Key: str("foo")},
			},
			want: true,
		},
		{
			name:       "build condition with step can use step variables",
			source:     upstreamBuildConditionSpec,
			expression: `step.key == "deploy"`,
			ctx: Context{
				EntryPoint: EntryPointBuildConditionWithStep,
				Step:       &Step{Key: str("deploy")},
			},
			want: true,
		},
		{
			name:       "step notification false when step condition fails",
			source:     upstreamStepNotificationSpec,
			expression: `step.id == "not-a-uuid"`,
			ctx: Context{
				EntryPoint: EntryPointStepNotification,
				Step:       &Step{ID: str("uuid")},
			},
			want: false,
		},
		{
			name:       "step notification false when unparseable",
			source:     upstreamStepNotificationSpec,
			expression: `nope != == one`,
			ctx:        Context{EntryPoint: EntryPointStepNotification},
			want:       false,
		},
	}

	runEvaluateCases(t, tests)
}
