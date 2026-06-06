// Package conformance contains source-tagged Buildkite conditional cases used by
// local tests and optional server-oracle tooling.
package conformance

import (
	"fmt"

	"github.com/buildkite/conditional"
)

// Mode identifies which root API operation a conformance case exercises.
type Mode string

const (
	// ModeEvaluate evaluates an expression and expects a boolean or error kind.
	ModeEvaluate Mode = "evaluate"
	// ModeValidate validates an expression and expects success or an error kind.
	ModeValidate Mode = "validate"
)

// Case is one Buildkite conditional conformance case.
type Case struct {
	Name       string                `json:"name"`
	Source     string                `json:"source"`
	Mode       Mode                  `json:"mode"`
	Expression string                `json:"expression"`
	Context    conditional.Context   `json:"context"`
	WantResult *bool                 `json:"want_result,omitempty"`
	WantError  conditional.ErrorKind `json:"want_error,omitempty"`
}

// Result is the normalized outcome returned by the local evaluator or an
// optional server oracle.
type Result struct {
	Result    *bool                 `json:"result,omitempty"`
	ErrorKind conditional.ErrorKind `json:"error_kind,omitempty"`
}

// OracleRequest is the JSON payload sent to an external server-oracle command.
type OracleRequest struct {
	Name       string              `json:"name"`
	Source     string              `json:"source"`
	Mode       Mode                `json:"mode"`
	Expression string              `json:"expression"`
	Context    conditional.Context `json:"context"`
}

// Cases returns the committed conformance corpus for optional server comparison.
func Cases() []Case {
	return []Case{
		{
			Name:       "docs branch is main or production",
			Source:     docsConditionalsSource,
			Mode:       ModeEvaluate,
			Expression: `build.branch == "main" || build.branch == "production"`,
			Context: conditional.Context{
				EntryPoint: conditional.EntryPointBuildCondition,
				Build:      conditional.Build{Branch: str("main")},
			},
			WantResult: boolptr(true),
		},
		{
			Name:       "docs feature branch regex",
			Source:     docsConditionalsSource,
			Mode:       ModeEvaluate,
			Expression: `build.branch =~ /^features\//`,
			Context: conditional.Context{
				EntryPoint: conditional.EntryPointBuildCondition,
				Build:      conditional.Build{Branch: str("features/api")},
			},
			WantResult: boolptr(true),
		},
		{
			Name:       "docs tag regex via build env",
			Source:     docsConditionalsSource,
			Mode:       ModeEvaluate,
			Expression: `build.env("BUILDKITE_TAG") =~ /^v[0-9]+\.0$/`,
			Context: conditional.Context{
				EntryPoint: conditional.EntryPointBuildCondition,
				Build:      conditional.Build{Tag: str("v2.0")},
			},
			WantResult: boolptr(true),
		},
		{
			Name:       "docs custom build env",
			Source:     docsConditionalsSource,
			Mode:       ModeEvaluate,
			Expression: `build.env("CUSTOM_ENVIRONMENT_VARIABLE") == "value"`,
			Context: conditional.Context{
				EntryPoint: conditional.EntryPointBuildCondition,
				BuildEnv:   map[string]string{"CUSTOM_ENVIRONMENT_VARIABLE": "value"},
			},
			WantResult: boolptr(true),
		},
		{
			Name:       "docs creator teams includes deploy",
			Source:     docsConditionalsSource,
			Mode:       ModeEvaluate,
			Expression: `build.creator.teams includes "deploy"`,
			Context: conditional.Context{
				EntryPoint: conditional.EntryPointBuildCondition,
				Build: conditional.Build{
					Creator: conditional.Actor{Teams: []string{"deploy", "platform"}},
				},
			},
			WantResult: boolptr(true),
		},
		{
			Name:       "docs merge queue base branch",
			Source:     docsConditionalsSource,
			Mode:       ModeEvaluate,
			Expression: `build.merge_queue.base_branch == "main"`,
			Context: conditional.Context{
				EntryPoint: conditional.EntryPointBuildCondition,
				Build: conditional.Build{
					MergeQueue: conditional.MergeQueue{BaseBranch: str("main")},
				},
			},
			WantResult: boolptr(true),
		},
		{
			Name:       "upstream null regex match is false",
			Source:     upstreamEvaluatorSpec,
			Mode:       ModeEvaluate,
			Expression: `null =~ /main|development/`,
			Context:    conditional.Context{EntryPoint: conditional.EntryPointBuildCondition},
			WantResult: boolptr(false),
		},
		{
			Name:       "upstream shell substitution default",
			Source:     upstreamEvaluatorSpec,
			Mode:       ModeEvaluate,
			Expression: `${notset:-fallback} == "fallback"`,
			Context:    conditional.Context{EntryPoint: conditional.EntryPointBuildCondition},
			WantResult: boolptr(true),
		},
		{
			Name:       "upstream ternary alternative branch",
			Source:     upstreamEvaluatorSpec,
			Mode:       ModeEvaluate,
			Expression: `1 == 2 ? 3 == 4 : 5 == 5`,
			Context:    conditional.Context{EntryPoint: conditional.EntryPointBuildCondition},
			WantResult: boolptr(true),
		},
		{
			Name:       "build notification parse errors are false",
			Source:     upstreamBuildNotificationSpec,
			Mode:       ModeEvaluate,
			Expression: `nope != == one`,
			Context:    conditional.Context{EntryPoint: conditional.EntryPointBuildNotification},
			WantResult: boolptr(false),
		},
		{
			Name:       "validate rejects unsupported Buildkite env",
			Source:     upstreamBuildConditionSpec,
			Mode:       ModeValidate,
			Expression: `build.env("BUILDKITE_AGENT_ACCESS_TOKEN") == null`,
			Context:    conditional.Context{EntryPoint: conditional.EntryPointBuildCondition},
			WantError:  conditional.ErrorKindValidation,
		},
		{
			Name:       "validate rejects step variables without step entrypoint",
			Source:     upstreamBuildValidatorSpec,
			Mode:       ModeValidate,
			Expression: `step.outcome == "hard_failed"`,
			Context:    conditional.Context{EntryPoint: conditional.EntryPointBuildCondition},
			WantError:  conditional.ErrorKindValidation,
		},
	}
}

// Expected returns the normalized expected result for a case.
func Expected(c Case) Result {
	if c.WantError != "" {
		return Result{ErrorKind: c.WantError}
	}
	return Result{Result: c.WantResult}
}

// OracleRequestFor returns the request payload for a case.
func OracleRequestFor(c Case) OracleRequest {
	return OracleRequest{
		Name:       c.Name,
		Source:     c.Source,
		Mode:       c.Mode,
		Expression: c.Expression,
		Context:    c.Context,
	}
}

// EvaluateLocal evaluates a case through the local root package API.
func EvaluateLocal(c Case) Result {
	switch c.Mode {
	case ModeEvaluate:
		got, err := conditional.Evaluate(c.Expression, c.Context)
		if err != nil {
			return Result{ErrorKind: errorKind(err)}
		}
		return Result{Result: boolptr(got)}
	case ModeValidate:
		if err := conditional.Validate(c.Expression, c.Context); err != nil {
			return Result{ErrorKind: errorKind(err)}
		}
		return Result{}
	default:
		return Result{ErrorKind: conditional.ErrorKindValidation}
	}
}

// Compare reports whether an actual result matches the expected result for mode.
func Compare(mode Mode, expected, actual Result) error {
	if actual.ErrorKind != "" && actual.Result != nil {
		return fmt.Errorf("result must be empty when error_kind is %q", actual.ErrorKind)
	}
	if expected.ErrorKind != actual.ErrorKind {
		return fmt.Errorf("error kind = %q, want %q", actual.ErrorKind, expected.ErrorKind)
	}
	if expected.ErrorKind != "" {
		return nil
	}
	switch mode {
	case ModeEvaluate:
		if expected.Result == nil || actual.Result == nil {
			return fmt.Errorf("result = %v, want %v", actual.Result, expected.Result)
		}
		if *actual.Result != *expected.Result {
			return fmt.Errorf("result = %t, want %t", *actual.Result, *expected.Result)
		}
	case ModeValidate:
		if actual.Result != nil {
			return fmt.Errorf("validation returned result %t", *actual.Result)
		}
	default:
		return fmt.Errorf("unsupported mode %q", mode)
	}
	return nil
}

func errorKind(err error) conditional.ErrorKind {
	for _, kind := range []conditional.ErrorKind{
		conditional.ErrorKindParse,
		conditional.ErrorKindValidation,
		conditional.ErrorKindEvaluation,
		conditional.ErrorKindResult,
	} {
		if conditional.IsErrorKind(err, kind) {
			return kind
		}
	}
	return conditional.ErrorKindEvaluation
}

const (
	docsConditionalsSource        = "https://buildkite.com/docs/pipelines/configure/conditionals"
	upstreamEvaluatorSpec         = "buildkite/buildkite:spec/models/conditional/evaluator_spec.rb"
	upstreamBuildConditionSpec    = "buildkite/buildkite:spec/models/build/condition_spec.rb"
	upstreamBuildValidatorSpec    = "buildkite/buildkite:spec/validators/build_condition_validator_spec.rb"
	upstreamBuildNotificationSpec = "buildkite/buildkite:spec/models/build/notification_spec.rb"
)

func str(value string) *string {
	return &value
}

func boolptr(value bool) *bool {
	return &value
}
