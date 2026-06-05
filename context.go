package conditional

// EntryPoint identifies the server path that is evaluating a conditional.
type EntryPoint string

const (
	// EntryPointBuildCondition evaluates a Build::Condition without a step.
	EntryPointBuildCondition EntryPoint = "build_condition"
	// EntryPointBuildConditionWithStep evaluates a Build::Condition with a step.
	EntryPointBuildConditionWithStep EntryPoint = "build_condition_with_step"
	// EntryPointBuildNotification evaluates build notification deliverability.
	EntryPointBuildNotification EntryPoint = "build_notification"
	// EntryPointStepNotification evaluates step notification deliverability.
	EntryPointStepNotification EntryPoint = "step_notification"
)

// Context contains the Buildkite values available to a conditional.
type Context struct {
	EntryPoint EntryPoint

	Build        Build
	Pipeline     Pipeline
	Organization Organization
	Step         *Step

	// BuildEnv is build-scoped environment. ProjectEnv is pipeline/project
	// environment. Matching Build::PipelineEnvironment, ProjectEnv is applied
	// first, then BuildEnv overrides it.
	BuildEnv   map[string]string
	ProjectEnv map[string]string
}

// Build contains build values exposed to conditionals.
type Build struct {
	ID           *string
	State        *string
	Fixed        *bool
	BlockedState *string
	Source       *string
	SourceEvent  *string
	SourceAction *string
	Branch       *string
	Tag          *string
	Message      *string
	Commit       *string
	Number       *int

	Creator     Actor
	Author      Actor
	SCM         SCM
	PullRequest PullRequest
	MergeQueue  MergeQueue
}

// Actor contains server-resolved author or creator values. Email should contain
// the value exposed through the server's build.*.email assignments, including
// organization-preferred creator email resolution when applicable.
type Actor struct {
	ID       *string
	Name     *string
	Email    *string
	Teams    []string
	Verified *bool
}

// Pipeline contains pipeline values exposed to conditionals.
type Pipeline struct {
	ID                      *string
	Name                    *string
	Slug                    *string
	DefaultBranch           *string
	Repository              *string
	StartedPassing          *bool
	StartedFailing          *bool
	NextFinishedBuildExists *bool
}

// SCM contains source control author and committer values.
type SCM struct {
	AuthorName     *string
	AuthorEmail    *string
	CommitterName  *string
	CommitterEmail *string
}

// PullRequest contains pull request values exposed to conditionals.
type PullRequest struct {
	ID             *string
	BaseBranch     *string
	Draft          *bool
	Label          *string
	Labels         []string
	Repository     *string
	RepositoryFork *bool
}

// MergeQueue contains merge queue values exposed to conditionals.
type MergeQueue struct {
	BaseBranch *string
	BaseCommit *string
}

// Organization contains organization values exposed to conditionals.
type Organization struct {
	ID   *string
	Slug *string
}

// Step contains step values exposed to step-aware conditionals.
type Step struct {
	ID      *string
	Key     *string
	Type    *string
	Label   *string
	State   *string
	Outcome *string
}
