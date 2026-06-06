package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	conditional "github.com/buildkite/conditional"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	ctx := processContext()

	for {
		io.WriteString(out, PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		if line == `quit` || line == `exit` {
			return
		}

		evaluated, err := conditional.Evaluate(line, ctx)
		if err != nil {
			fmt.Fprintf(out, "ERROR: %s\n", err)
			continue
		}
		fmt.Fprintf(out, "%t\n", evaluated)
	}
}

func processEnv() map[string]string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func processContext() conditional.Context {
	return contextFromEnv(processEnv())
}

func contextFromEnv(env map[string]string) conditional.Context {
	return conditional.Context{
		EntryPoint: conditional.EntryPointBuildCondition,
		BuildEnv:   env,
		Build: conditional.Build{
			Branch:  stringEnv(env, "BUILDKITE_BRANCH"),
			Tag:     stringEnv(env, "BUILDKITE_TAG"),
			Message: stringEnv(env, "BUILDKITE_MESSAGE"),
			Commit:  stringEnv(env, "BUILDKITE_COMMIT"),
			Source:  sourceEnv(env),
			PullRequest: conditional.PullRequest{
				ID:                pullRequestEnv(env),
				BaseBranch:        stringEnv(env, "BUILDKITE_PULL_REQUEST_BASE_BRANCH"),
				Labels:            pullRequestLabelsEnv(env),
				Repository:        stringEnv(env, "BUILDKITE_PULL_REQUEST_REPO"),
				UsingMergeRefspec: boolEnv(env, "BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC"),
			},
			MergeQueue: mergeQueueEnv(env),
			TriggeredFrom: conditional.TriggeredFrom{
				BuildID:      stringEnv(env, "BUILDKITE_TRIGGERED_FROM_BUILD_ID"),
				BuildNumber:  intEnv(env, "BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER"),
				PipelineSlug: stringEnv(env, "BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG"),
				JobID:        stringEnv(env, "BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID"),
			},
			RebuiltFrom: conditional.RebuiltFrom{
				BuildID:     stringEnv(env, "BUILDKITE_REBUILT_FROM_BUILD_ID"),
				BuildNumber: intEnv(env, "BUILDKITE_REBUILT_FROM_BUILD_NUMBER"),
			},
		},
		Pipeline: conditional.Pipeline{
			Name:                                  stringEnv(env, "BUILDKITE_PIPELINE_NAME"),
			Slug:                                  stringEnv(env, "BUILDKITE_PIPELINE_SLUG"),
			ID:                                    stringEnv(env, "BUILDKITE_PIPELINE_ID"),
			Repository:                            stringEnv(env, "BUILDKITE_REPO"),
			UseMergeQueueBaseCommitForGitDiffBase: useMergeQueueBaseCommitEnv(env),
		},
		Organization: conditional.Organization{
			Slug: stringEnv(env, "BUILDKITE_ORGANIZATION_SLUG"),
		},
	}
}

func stringEnv(env map[string]string, key string) *string {
	value, ok := env[key]
	if !ok {
		return nil
	}
	return &value
}

func pullRequestEnv(env map[string]string) *string {
	value := stringEnv(env, "BUILDKITE_PULL_REQUEST")
	if value == nil || *value == "false" {
		return nil
	}
	return value
}

func pullRequestLabelsEnv(env map[string]string) []string {
	value, ok := env["BUILDKITE_PULL_REQUEST_LABELS"]
	if !ok {
		return nil
	}
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func sourceEnv(env map[string]string) *string {
	if _, ok := env["BUILDKITE_GITHUB_EVENT"]; !ok {
		return nil
	}
	source := "webhook"
	return &source
}

func mergeQueueEnv(env map[string]string) conditional.MergeQueue {
	gitDiffBase := stringEnv(env, "BUILDKITE_GIT_DIFF_BASE")
	baseBranch := stringEnv(env, "BUILDKITE_MERGE_QUEUE_BASE_BRANCH")
	baseCommit := stringEnv(env, "BUILDKITE_MERGE_QUEUE_BASE_COMMIT")
	if gitDiffBase != nil && baseBranch == nil && baseCommit == nil {
		baseBranch = gitDiffBase
	}
	return conditional.MergeQueue{
		Active:     gitDiffBase != nil,
		BaseBranch: baseBranch,
		BaseCommit: baseCommit,
	}
}

func useMergeQueueBaseCommitEnv(env map[string]string) *bool {
	gitDiffBase := stringEnv(env, "BUILDKITE_GIT_DIFF_BASE")
	baseCommit := stringEnv(env, "BUILDKITE_MERGE_QUEUE_BASE_COMMIT")
	if gitDiffBase == nil || baseCommit == nil {
		return nil
	}
	useBaseCommit := *gitDiffBase == *baseCommit
	return &useBaseCommit
}

func boolEnv(env map[string]string, key string) *bool {
	value, ok := env[key]
	if !ok || value == "" {
		return nil
	}
	parsed := value == "true"
	return &parsed
}

func intEnv(env map[string]string, key string) *int {
	value, ok := env[key]
	if !ok || value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}
