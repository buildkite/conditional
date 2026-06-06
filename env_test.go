package conditional

import (
	"fmt"
	"slices"
	"testing"
)

var serverBuildkiteEnvAllowlist = []struct {
	name string
	key  string
}{
	{name: "branch", key: "BUILDKITE_BRANCH"},
	{name: "tag", key: "BUILDKITE_TAG"},
	{name: "message", key: "BUILDKITE_MESSAGE"},
	{name: "commit", key: "BUILDKITE_COMMIT"},
	{name: "pipeline slug", key: "BUILDKITE_PIPELINE_SLUG"},
	{name: "pipeline name", key: "BUILDKITE_PIPELINE_NAME"},
	{name: "pipeline id", key: "BUILDKITE_PIPELINE_ID"},
	{name: "organization slug", key: "BUILDKITE_ORGANIZATION_SLUG"},
	{name: "triggered from build id", key: "BUILDKITE_TRIGGERED_FROM_BUILD_ID"},
	{name: "triggered from build number", key: "BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER"},
	{name: "triggered from build pipeline slug", key: "BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG"},
	{name: "triggered from build job id", key: "BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID"},
	{name: "rebuilt from build id", key: "BUILDKITE_REBUILT_FROM_BUILD_ID"},
	{name: "rebuilt from build number", key: "BUILDKITE_REBUILT_FROM_BUILD_NUMBER"},
	{name: "repo", key: "BUILDKITE_REPO"},
	{name: "pull request", key: "BUILDKITE_PULL_REQUEST"},
	{name: "pull request base branch", key: "BUILDKITE_PULL_REQUEST_BASE_BRANCH"},
	{name: "pull request repo", key: "BUILDKITE_PULL_REQUEST_REPO"},
	{name: "pull request using merge refspec", key: "BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC"},
	{name: "merge queue base branch", key: "BUILDKITE_MERGE_QUEUE_BASE_BRANCH"},
	{name: "merge queue base commit", key: "BUILDKITE_MERGE_QUEUE_BASE_COMMIT"},
	{name: "git diff base", key: "BUILDKITE_GIT_DIFF_BASE"},
	{name: "github action", key: "BUILDKITE_GITHUB_ACTION"},
	{name: "github comment id", key: "BUILDKITE_GITHUB_COMMENT_ID"},
	{name: "github deployment id", key: "BUILDKITE_GITHUB_DEPLOYMENT_ID"},
	{name: "github deployment task", key: "BUILDKITE_GITHUB_DEPLOYMENT_TASK"},
	{name: "github deployment environment", key: "BUILDKITE_GITHUB_DEPLOYMENT_ENVIRONMENT"},
	{name: "github deployment payload", key: "BUILDKITE_GITHUB_DEPLOYMENT_PAYLOAD"},
	{name: "github event", key: "BUILDKITE_GITHUB_EVENT"},
	{name: "github review id", key: "BUILDKITE_GITHUB_REVIEW_ID"},
	{name: "github check run conclusion", key: "BUILDKITE_GITHUB_CHECK_RUN_CONCLUSION"},
	{name: "github check run name", key: "BUILDKITE_GITHUB_CHECK_RUN_NAME"},
	{name: "github deployment status environment", key: "BUILDKITE_GITHUB_DEPLOYMENT_STATUS_ENVIRONMENT"},
	{name: "github deployment status state", key: "BUILDKITE_GITHUB_DEPLOYMENT_STATUS_STATE"},
	{name: "github release draft", key: "BUILDKITE_GITHUB_RELEASE_DRAFT"},
	{name: "github release prerelease", key: "BUILDKITE_GITHUB_RELEASE_PRERELEASE"},
	{name: "github release tag", key: "BUILDKITE_GITHUB_RELEASE_TAG"},
	{name: "github review state", key: "BUILDKITE_GITHUB_REVIEW_STATE"},
}

func TestServerBuildkiteEnvAllowlistMatchesImplementation(t *testing.T) {
	requireCaseSource(t, upstreamBuildPipelineEnvModel)

	want := make([]string, 0, len(serverBuildkiteEnvAllowlist))
	for _, tt := range serverBuildkiteEnvAllowlist {
		want = append(want, tt.key)
	}
	if !slices.Equal(supportedBuildkiteEnvNames, want) {
		t.Fatalf("supportedBuildkiteEnvNames = %#v, want %#v", supportedBuildkiteEnvNames, want)
	}
}

func TestServerBuildkiteEnvAllowlistValidates(t *testing.T) {
	for _, tt := range serverBuildkiteEnvAllowlist {
		for _, function := range []string{"env", "build.env"} {
			t.Run(tt.name+" "+function, func(t *testing.T) {
				requireCaseSource(t, upstreamBuildPipelineEnvModel)

				expression := fmt.Sprintf(`%s(%q) != null`, function, tt.key)
				err := Validate(expression, Context{EntryPoint: EntryPointBuildCondition})
				if err != nil {
					t.Fatalf("Validate(%q) returned error: %v", expression, err)
				}
			})
		}
	}
}

func TestDocumentedGitHubBuildkiteEnvValuesComeFromBuildEnv(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "github deployment id", key: "BUILDKITE_GITHUB_DEPLOYMENT_ID"},
		{name: "github deployment task", key: "BUILDKITE_GITHUB_DEPLOYMENT_TASK"},
		{name: "github deployment environment", key: "BUILDKITE_GITHUB_DEPLOYMENT_ENVIRONMENT"},
		{name: "github deployment payload", key: "BUILDKITE_GITHUB_DEPLOYMENT_PAYLOAD"},
		{name: "github action", key: "BUILDKITE_GITHUB_ACTION"},
		{name: "github check run conclusion", key: "BUILDKITE_GITHUB_CHECK_RUN_CONCLUSION"},
		{name: "github comment id", key: "BUILDKITE_GITHUB_COMMENT_ID"},
		{name: "github check run name", key: "BUILDKITE_GITHUB_CHECK_RUN_NAME"},
		{name: "github deployment status environment", key: "BUILDKITE_GITHUB_DEPLOYMENT_STATUS_ENVIRONMENT"},
		{name: "github deployment status state", key: "BUILDKITE_GITHUB_DEPLOYMENT_STATUS_STATE"},
		{name: "github event", key: "BUILDKITE_GITHUB_EVENT"},
		{name: "github release draft", key: "BUILDKITE_GITHUB_RELEASE_DRAFT"},
		{name: "github release prerelease", key: "BUILDKITE_GITHUB_RELEASE_PRERELEASE"},
		{name: "github release tag", key: "BUILDKITE_GITHUB_RELEASE_TAG"},
		{name: "github review id", key: "BUILDKITE_GITHUB_REVIEW_ID"},
		{name: "github review state", key: "BUILDKITE_GITHUB_REVIEW_STATE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireCaseSource(t, docsConditionalsSource)

			value := "from-build-env"
			expression := fmt.Sprintf(`env(%[1]q) == %[2]q && build.env(%[1]q) == %[2]q`, tt.key, value)
			got, err := Evaluate(expression, Context{
				EntryPoint: EntryPointBuildCondition,
				BuildEnv:   map[string]string{tt.key: value},
			})
			if err != nil {
				t.Fatalf("Evaluate(%q) returned error: %v", expression, err)
			}
			if !got {
				t.Fatalf("Evaluate(%q) = false, want true", expression)
			}
		})
	}
}
