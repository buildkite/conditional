package repl

import (
	"bytes"
	"strings"
	"testing"

	conditional "github.com/buildkite/conditional"
)

func TestStartEvaluatesThroughRootPackage(t *testing.T) {
	var out bytes.Buffer

	Start(strings.NewReader("true\nfalse\nexit\n"), &out)

	want := ">> true\n>> false\n>> "
	if out.String() != want {
		t.Fatalf("Start output = %q, want %q", out.String(), want)
	}
}

func TestStartPrintsRootErrors(t *testing.T) {
	var out bytes.Buffer

	Start(strings.NewReader("step.key == \"deploy\"\nexit\n"), &out)

	want := ">> ERROR: validation: step variables are not available for entry point \"build_condition\"\n>> "
	if out.String() != want {
		t.Fatalf("Start output = %q, want %q", out.String(), want)
	}
}

func TestContextFromEnvPreservesBuildkiteBuiltins(t *testing.T) {
	ctx := contextFromEnv(map[string]string{
		"BUILDKITE_BRANCH":                             "main",
		"BUILDKITE_TAG":                                "v1.2.3",
		"BUILDKITE_MESSAGE":                            "ship it",
		"BUILDKITE_COMMIT":                             "abc123",
		"BUILDKITE_PIPELINE_NAME":                      "Deploy",
		"BUILDKITE_PIPELINE_SLUG":                      "deploy",
		"BUILDKITE_PIPELINE_ID":                        "018f",
		"BUILDKITE_REPO":                               "git@github.com:acme/repo.git",
		"BUILDKITE_ORGANIZATION_SLUG":                  "acme",
		"BUILDKITE_PULL_REQUEST":                       "123",
		"BUILDKITE_PULL_REQUEST_BASE_BRANCH":           "main",
		"BUILDKITE_PULL_REQUEST_REPO":                  "git@github.com:acme/repo.git",
		"BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC":   "true",
		"BUILDKITE_MERGE_QUEUE_BASE_BRANCH":            "main",
		"BUILDKITE_MERGE_QUEUE_BASE_COMMIT":            "def456",
		"BUILDKITE_GIT_DIFF_BASE":                      "def456",
		"BUILDKITE_PULL_REQUEST_LABELS":                "bug,deploy",
		"LABELS_KEY":                                   "BUILDKITE_PULL_REQUEST_LABELS",
		"BUILDKITE_TRIGGERED_FROM_BUILD_ID":            "triggered",
		"BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER":        "42",
		"BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG": "deploy",
		"BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID":        "job",
		"BUILDKITE_REBUILT_FROM_BUILD_ID":              "rebuilt",
		"BUILDKITE_REBUILT_FROM_BUILD_NUMBER":          "41",
		"BUILDKITE_GITHUB_EVENT":                       "pull_request",
		"BUILDKITE_GITHUB_ACTION":                      "labeled",
	})

	tests := []string{
		`build.branch == "main"`,
		`build.env("BUILDKITE_TAG") == "v1.2.3"`,
		`build.env("BUILDKITE_MESSAGE") == "ship it"`,
		`build.env("BUILDKITE_COMMIT") == "abc123"`,
		`build.env("BUILDKITE_PIPELINE_NAME") == "Deploy"`,
		`build.env("BUILDKITE_PIPELINE_SLUG") == "deploy"`,
		`build.env("BUILDKITE_PIPELINE_ID") == "018f"`,
		`build.env("BUILDKITE_REPO") == "git@github.com:acme/repo.git"`,
		`build.env("BUILDKITE_ORGANIZATION_SLUG") == "acme"`,
		`build.env("BUILDKITE_PULL_REQUEST") == "123"`,
		`build.env("BUILDKITE_PULL_REQUEST_BASE_BRANCH") == "main"`,
		`build.env("BUILDKITE_PULL_REQUEST_REPO") == "git@github.com:acme/repo.git"`,
		`build.env("BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC") == "true"`,
		`build.env(env("LABELS_KEY")) == "bug,deploy"`,
		`build.env("BUILDKITE_MERGE_QUEUE_BASE_BRANCH") == "main"`,
		`build.env("BUILDKITE_MERGE_QUEUE_BASE_COMMIT") == "def456"`,
		`build.env("BUILDKITE_GIT_DIFF_BASE") == "def456"`,
		`build.env("BUILDKITE_TRIGGERED_FROM_BUILD_ID") == "triggered"`,
		`build.env("BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER") == "42"`,
		`build.env("BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG") == "deploy"`,
		`build.env("BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID") == "job"`,
		`build.env("BUILDKITE_REBUILT_FROM_BUILD_ID") == "rebuilt"`,
		`build.env("BUILDKITE_REBUILT_FROM_BUILD_NUMBER") == "41"`,
		`build.source_event == "pull_request"`,
		`build.source_action == "labeled"`,
	}

	for _, expression := range tests {
		got, err := conditional.Evaluate(expression, ctx)
		if err != nil {
			t.Fatalf("Evaluate(%q) returned error: %v", expression, err)
		}
		if !got {
			t.Fatalf("Evaluate(%q) = false, want true", expression)
		}
	}
}
