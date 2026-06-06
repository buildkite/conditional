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

func TestBuildkiteContextFullAssignmentMatrix(t *testing.T) {
	ctx := Context{
		EntryPoint: EntryPointBuildCondition,
		Build: Build{
			ID:           str("build-uuid"),
			State:        str("scheduled"),
			Fixed:        boolptr(false),
			BlockedState: str("passed"),
			Source:       str("webhook"),
			SourceEvent:  str("pull_request"),
			SourceAction: str("labeled"),
			Branch:       str("the-branch"),
			Tag:          str("the-tag"),
			Message:      str("the-message"),
			Commit:       str("the-commit"),
			Number:       intptr(1234),
			Creator: Actor{
				ID:       str("created-by-uuid"),
				Name:     str("Created By Name"),
				Email:    str("created_by@email.com"),
				Verified: boolptr(false),
			},
			Author: Actor{
				ID:    str("authored-by-uuid"),
				Name:  str("Authored By Name"),
				Email: str("authored_by@email.com"),
			},
			SCM: SCM{
				AuthorName:     str("Samantha Carter"),
				AuthorEmail:    str("sam@sgc.gov"),
				CommitterName:  str("GitHub"),
				CommitterEmail: str("noreply@github.com"),
			},
			PullRequest: PullRequest{
				ID:             str("1234"),
				BaseBranch:     str("base-branch-party"),
				Draft:          boolptr(false),
				Label:          str("test-gpu"),
				Labels:         []string{"bug", "feature", "duplicate"},
				Repository:     str("git@foo.com"),
				RepositoryFork: boolptr(true),
			},
			MergeQueue: MergeQueue{
				BaseBranch: str("merge-base-branch"),
				BaseCommit: str("merge-base-commit"),
			},
		},
		Pipeline: Pipeline{
			ID:                      str("pipeline-uuid"),
			Slug:                    str("pipeline-slug"),
			DefaultBranch:           str("main"),
			Repository:              str("git@github.com:acme/repo.git"),
			StartedPassing:          boolptr(false),
			StartedFailing:          boolptr(false),
			NextFinishedBuildExists: boolptr(true),
			Name:                    str("Deploy"),
		},
		Organization: Organization{
			ID:   str("organization-uuid"),
			Slug: str("organization-slug"),
		},
	}

	tests := []evaluateCase{
		{
			name:       "build scalar assignments",
			source:     upstreamBuildConditionSpec,
			expression: `build.id == "build-uuid" && build.state == "scheduled" && !build.fixed && build.blocked_state == "passed" && build.source == "webhook" && build.branch == "the-branch" && build.tag == "the-tag" && build.message == "the-message" && build.commit == "the-commit" && build.number == 1234`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "webhook source assignments",
			source:     upstreamBuildConditionSpec,
			expression: `build.source_event == "pull_request" && build.source_action == "labeled" && build.pull_request.label == "test-gpu"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "creator assignments",
			source:     upstreamBuildConditionSpec,
			expression: `build.creator.id == "created-by-uuid" && build.creator.name == "Created By Name" && build.creator.email == "created_by@email.com" && build.creator.teams == null && !build.creator.verified`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "author assignments",
			source:     upstreamBuildConditionSpec,
			expression: `build.author.id == "authored-by-uuid" && build.author.name == "Authored By Name" && build.author.email == "authored_by@email.com" && build.author.teams == null`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "scm assignments",
			source:     upstreamBuildConditionSpec,
			expression: `build.scm.author.name == "Samantha Carter" && build.scm.author.email == "sam@sgc.gov" && build.scm.committer.name == "GitHub" && build.scm.committer.email == "noreply@github.com"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "pull request assignments",
			source:     upstreamBuildConditionSpec,
			expression: `build.pull_request.id == "1234" && build.pull_request.base_branch == "base-branch-party" && !build.pull_request.draft && build.pull_request.labels == ["bug", "feature", "duplicate"] && build.pull_request.repository == "git@foo.com" && build.pull_request.repository.fork`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "merge queue assignments",
			source:     upstreamBuildConditionSpec,
			expression: `build.merge_queue.base_branch == "merge-base-branch" && build.merge_queue.base_commit == "merge-base-commit"`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "pipeline assignments",
			source:     upstreamBuildConditionSpec,
			expression: `pipeline.id == "pipeline-uuid" && pipeline.slug == "pipeline-slug" && pipeline.default_branch == "main" && pipeline.repository == "git@github.com:acme/repo.git" && !pipeline.started_passing && !pipeline.started_failing && pipeline.next_finished_build_exists`,
			ctx:        ctx,
			want:       true,
		},
		{
			name:       "organization assignments",
			source:     upstreamBuildConditionSpec,
			expression: `organization.id == "organization-uuid" && organization.slug == "organization-slug"`,
			ctx:        ctx,
			want:       true,
		},
	}

	runEvaluateCases(t, tests)
}

func TestBuildkiteContextNullAssignmentMatrix(t *testing.T) {
	tests := []evaluateCase{
		{
			name:   "build scalar assignments are null without context values",
			source: upstreamBuildConditionSpec,
			expression: `build.id == null && build.state == null && build.fixed == null && build.blocked_state == null &&
				build.source == null && build.source_event == null && build.source_action == null &&
				build.branch == null && build.tag == null && build.message == null && build.commit == null && build.number == null`,
			ctx:  Context{EntryPoint: EntryPointBuildCondition},
			want: true,
		},
		{
			name:   "actor assignments are null without context values",
			source: upstreamBuildConditionSpec,
			expression: `build.creator.id == null && build.creator.name == null && build.creator.email == null &&
				build.creator.teams == null && build.creator.verified == null &&
				build.author.id == null && build.author.name == null && build.author.email == null && build.author.teams == null`,
			ctx:  Context{EntryPoint: EntryPointBuildCondition},
			want: true,
		},
		{
			name:   "pull request assignments are null without context values",
			source: upstreamBuildConditionSpec,
			expression: `build.pull_request.id == null && build.pull_request.base_branch == null &&
				build.pull_request.draft == null && build.pull_request.label == null &&
				build.pull_request.labels == null && build.pull_request.repository == null &&
				build.pull_request.repository.fork == null`,
			ctx:  Context{EntryPoint: EntryPointBuildCondition},
			want: true,
		},
		{
			name:   "pipeline and organization assignments are null without context values",
			source: upstreamBuildConditionSpec,
			expression: `pipeline.id == null && pipeline.slug == null && pipeline.default_branch == null &&
				pipeline.repository == null && pipeline.started_passing == null &&
				pipeline.started_failing == null && pipeline.next_finished_build_exists == null &&
				organization.id == null && organization.slug == null`,
			ctx:  Context{EntryPoint: EntryPointBuildCondition},
			want: true,
		},
		{
			name:       "merge queue assignments are null for non merge queue context values",
			source:     upstreamBuildConditionSpec,
			expression: `build.merge_queue.base_branch == null && build.merge_queue.base_commit == null`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			want:       true,
		},
		{
			name:   "scm assignments are null without context values",
			source: upstreamBuildConditionSpec,
			expression: `build.scm.author.name == null && build.scm.author.email == null &&
				build.scm.committer.name == null && build.scm.committer.email == null`,
			ctx:  Context{EntryPoint: EntryPointBuildCondition},
			want: true,
		},
	}

	runEvaluateCases(t, tests)
}

func TestBuildkiteActorContextAssignments(t *testing.T) {
	tests := []evaluateCase{
		{
			name:       "empty team arrays stay distinct from null teams",
			source:     upstreamBuildConditionSpec,
			expression: `build.creator.teams == [] && build.author.teams == []`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					Creator: Actor{Teams: []string{}},
					Author:  Actor{Teams: []string{}},
				},
			},
			want: true,
		},
		{
			name:       "only caller supplied visible teams are exposed",
			source:     upstreamBuildConditionSpec,
			expression: `build.creator.teams == ["created-by-team"] && build.author.teams == ["authored-by-team"] && !(build.author.teams includes "team-secret")`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					Creator: Actor{Teams: []string{"created-by-team"}},
					Author:  Actor{Teams: []string{"authored-by-team"}},
				},
			},
			want: true,
		},
		{
			name:       "verified creator and preferred emails are caller supplied context values",
			source:     upstreamBuildConditionSpec,
			expression: `build.creator.verified && build.creator.email == "created_by+preferred@email.com" && build.author.email == "authored_by@email.com"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					Creator: Actor{
						Email:    str("created_by+preferred@email.com"),
						Verified: boolptr(true),
					},
					Author: Actor{
						Email: str("authored_by@email.com"),
					},
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
			name:       "env accepts interpolated variable name",
			source:     upstreamBuildConditionSpec,
			expression: `env("${NAME}") == "value"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv: map[string]string{
					"NAME":        "DYNAMIC_ENV",
					"DYNAMIC_ENV": "value",
				},
			},
			want: true,
		},
		{
			name:       "build env accepts interpolated Buildkite variable name",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("BUILDKITE_${SUFFIX}") == "main"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build:      Build{Branch: str("main")},
				BuildEnv:   map[string]string{"SUFFIX": "BRANCH"},
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
			name:   "supported built in Buildkite env values default blank",
			source: upstreamBuildPipelineEnvModel,
			expression: `build.env("BUILDKITE_TRIGGERED_FROM_BUILD_ID") == "" &&
				build.env("BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER") == "" &&
				build.env("BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG") == "" &&
				build.env("BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID") == "" &&
				build.env("BUILDKITE_REBUILT_FROM_BUILD_ID") == "" &&
				build.env("BUILDKITE_REBUILT_FROM_BUILD_NUMBER") == "" &&
				build.env("BUILDKITE_PULL_REQUEST_LABELS") == "" &&
				build.env("BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC") == "" &&
				build.env("BUILDKITE_GIT_DIFF_BASE") == ""`,
			ctx:  Context{EntryPoint: EntryPointBuildCondition},
			want: true,
		},
		{
			name:   "triggered and rebuilt built in Buildkite env values are derived",
			source: upstreamBuildPipelineEnvModel,
			expression: `build.env("BUILDKITE_TRIGGERED_FROM_BUILD_ID") == "triggered-build" &&
				build.env("BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER") == "42" &&
				build.env("BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG") == "deploy" &&
				build.env("BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID") == "trigger-job" &&
				build.env("BUILDKITE_REBUILT_FROM_BUILD_ID") == "rebuilt-build" &&
				build.env("BUILDKITE_REBUILT_FROM_BUILD_NUMBER") == "41"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					TriggeredFrom: TriggeredFrom{
						BuildID:      str("triggered-build"),
						BuildNumber:  intptr(42),
						PipelineSlug: str("deploy"),
						JobID:        str("trigger-job"),
					},
					RebuiltFrom: RebuiltFrom{
						BuildID:     str("rebuilt-build"),
						BuildNumber: intptr(41),
					},
				},
			},
			want: true,
		},
		{
			name:   "pull request merge refspec and git diff base values are derived",
			source: upstreamBuildPipelineEnvModel,
			expression: `build.env("BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC") == "true" &&
				build.env("BUILDKITE_GIT_DIFF_BASE") == "def456"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					PullRequest: PullRequest{UsingMergeRefspec: boolptr(true)},
					MergeQueue: MergeQueue{
						Active:     true,
						BaseBranch: str("main"),
						BaseCommit: str("def456"),
					},
				},
				Pipeline: Pipeline{UseMergeQueueBaseCommitForGitDiffBase: boolptr(true)},
			},
			want: true,
		},
		{
			name:   "git diff base uses merge queue base branch by default",
			source: upstreamBuildPipelineEnvModel,
			expression: `build.env("BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC") == "" &&
				build.env("BUILDKITE_GIT_DIFF_BASE") == "main"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					PullRequest: PullRequest{UsingMergeRefspec: boolptr(false)},
					MergeQueue: MergeQueue{
						Active:     true,
						BaseBranch: str("main"),
						BaseCommit: str("def456"),
					},
				},
			},
			want: true,
		},
		{
			name:   "git diff base is blank outside merge queue builds",
			source: upstreamBuildPipelineEnvModel,
			expression: `build.env("BUILDKITE_MERGE_QUEUE_BASE_BRANCH") == "main" &&
				build.env("BUILDKITE_MERGE_QUEUE_BASE_COMMIT") == "def456" &&
				build.env("BUILDKITE_GIT_DIFF_BASE") == ""`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Build: Build{
					MergeQueue: MergeQueue{
						BaseBranch: str("main"),
						BaseCommit: str("def456"),
					},
				},
			},
			want: true,
		},
		{
			name:       "indirect unsupported Buildkite env fails closed",
			source:     upstreamBuildConditionSpec,
			expression: `build.env(env("SECRET_NAME")) == null`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv: map[string]string{
					"SECRET_NAME":                  "BUILDKITE_AGENT_ACCESS_TOKEN",
					"BUILDKITE_AGENT_ACCESS_TOKEN": "secret",
				},
			},
			wantError: ErrorKindEvaluation,
		},
		{
			name:       "indirect blank env name fails closed",
			source:     upstreamBuildConditionSpec,
			expression: `env(env("SECRET_NAME")) == ""`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv:   map[string]string{"SECRET_NAME": ""},
			},
			wantError: ErrorKindEvaluation,
		},
		{
			name:       "indirect custom env name uses runtime lookup",
			source:     upstreamBuildPipelineEnvModel,
			expression: `env(env("SECRET_NAME")) == "value" && build.env(env("SECRET_NAME")) == "value"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv: map[string]string{
					"SECRET_NAME": "FOO-BAR",
					"FOO-BAR":     "value",
				},
			},
			want: true,
		},
		{
			name:       "missing indirect env name fails closed",
			source:     upstreamBuildConditionSpec,
			expression: `build.env(env("MISSING_SECRET_NAME")) == null`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindEvaluation,
		},
		{
			name:       "build notification false for indirect unsupported env",
			source:     upstreamBuildNotificationSpec,
			expression: `env(env("SECRET_NAME")) == "secret"`,
			ctx: Context{
				EntryPoint: EntryPointBuildNotification,
				BuildEnv: map[string]string{
					"SECRET_NAME":                  "BUILDKITE_AGENT_ACCESS_TOKEN",
					"BUILDKITE_AGENT_ACCESS_TOKEN": "secret",
				},
			},
			want: false,
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
			wantError:  ErrorKindValidation,
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
			name:       "pipeline name is not a server conditional variable",
			source:     upstreamBuildConditionSpec,
			expression: `pipeline.name == "Deploy"`,
			ctx: Context{
				EntryPoint: EntryPointBuildCondition,
				Pipeline:   Pipeline{Name: str("Deploy")},
			},
			wantError: ErrorKindValidation,
		},
		{
			name:       "literal unsupported Buildkite env is rejected",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("BUILDKITE_AGENT_ACCESS_TOKEN") == null`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
			wantMessageContains: []string{
				`Interpolation of "BUILDKITE_AGENT_ACCESS_TOKEN" is not supported`,
				`runtime`,
			},
		},
		{
			name:       "literal Buildkite env typo suggests supported name",
			source:     upstreamBuildConditionSpec,
			expression: `env("BUILDKITE_MESSGE") == "blah"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
			wantMessageContains: []string{
				`"BUILDKITE_MESSGE" is not a valid environment variable`,
				`did you mean "BUILDKITE_MESSAGE"`,
			},
		},
		{
			name:       "literal build env typo suggests supported name",
			source:     upstreamBuildConditionSpec,
			expression: `build.env("BUILDKITE_MESSGE") == null`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
			wantMessageContains: []string{
				`"BUILDKITE_MESSGE" is not a valid environment variable`,
				`did you mean "BUILDKITE_MESSAGE"`,
			},
		},
		{
			name:       "literal invalid env name uses server invalid name error",
			source:     upstreamBuildConditionSpec,
			expression: `env("!") == "blah"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
			wantMessageContains: []string{
				"Argument to `env` should be an environment variable name",
			},
		},
		{
			name:       "literal env name starting with dollar suggests removing dollars",
			source:     upstreamBuildConditionSpec,
			expression: `env("$$BUILDKITE_MESSAGE") == "blah"`,
			ctx:        Context{EntryPoint: EntryPointBuildCondition},
			wantError:  ErrorKindValidation,
			wantMessageContains: []string{
				"Argument to `env` should not include `$`",
				"did you mean BUILDKITE_MESSAGE",
			},
		},
	}

	runValidateCases(t, tests)
}

func TestBuildkiteEnvSuggestions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "message typo",
			input: "BUILDKITE_MESSGE",
			want:  "BUILDKITE_MESSAGE",
		},
		{
			name:  "unsupported build number does not suggest",
			input: "BUILDKITE_BUILD_NUMBER",
		},
		{
			name:  "github review typo picks first server suggestion",
			input: "BUILDKITE_GITHUB_REVIE_STATE",
			want:  "BUILDKITE_GITHUB_REVIEW_STATE",
		},
		{
			name:  "pull request label singular picks labels",
			input: "BUILDKITE_PULL_REQUEST_LABEL",
			want:  "BUILDKITE_PULL_REQUEST_LABELS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireCaseSource(t, upstreamBuildkiteSuggestion)

			if got := suggestBuildkiteEnv(tt.input); got != tt.want {
				t.Fatalf("suggestBuildkiteEnv(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
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
			name:       "step notification accepts waiting for input state",
			source:     upstreamStepStateMachineModel,
			expression: `step.state == "waiting_for_input"`,
			ctx: Context{
				EntryPoint: EntryPointStepNotification,
				Step:       &Step{State: str("waiting_for_input")},
			},
			want: true,
		},
		{
			name:       "step notification accepts canceled state",
			source:     upstreamStepStateMachineModel,
			expression: `step.state == "canceled"`,
			ctx: Context{
				EntryPoint: EntryPointStepNotification,
				Step:       &Step{State: str("canceled")},
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
