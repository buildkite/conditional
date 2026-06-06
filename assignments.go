package conditional

import "github.com/buildkite/conditional/internal/object"

type assignmentDefinition struct {
	name  string
	typ   valueType
	value func(Context) object.Object
}

var baseAssignmentDefinitions = []assignmentDefinition{
	{name: "build.id", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.ID) }},
	{name: "build.state", typ: enumValueType("build state", "creating", "started", "running", "scheduled", "blocked", "passed", "failing", "failed", "canceling", "canceled", "skipped", "not_run"), value: func(ctx Context) object.Object { return stringValue(ctx.Build.State) }},
	{name: "build.fixed", typ: boolType(), value: func(ctx Context) object.Object { return boolValue(ctx.Build.Fixed) }},
	{name: "build.blocked_state", typ: enumValueType("build blocked state", "failed", "passed", "running"), value: func(ctx Context) object.Object { return stringValue(ctx.Build.BlockedState) }},
	{name: "build.source", typ: enumValueType("build source", "api", "ui", "webhook", "trigger_job", "schedule", "pipeline_trigger"), value: func(ctx Context) object.Object { return stringValue(ctx.Build.Source) }},
	{name: "build.source_event", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(sourceEvent(ctx)) }},
	{name: "build.source_action", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(sourceAction(ctx)) }},
	{name: "build.branch", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.Branch) }},
	{name: "build.tag", typ: stringType(), value: func(ctx Context) object.Object { return presenceStringValue(ctx.Build.Tag) }},
	{name: "build.message", typ: stringType(), value: func(ctx Context) object.Object { return presenceStringValue(ctx.Build.Message) }},
	{name: "build.commit", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.Commit) }},
	{name: "build.number", typ: numberType(), value: func(ctx Context) object.Object { return intValue(ctx.Build.Number) }},
	{name: "build.creator.id", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.Creator.ID) }},
	{name: "build.creator.name", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.Creator.Name) }},
	{name: "build.creator.email", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.Creator.Email) }},
	{name: "build.creator.teams", typ: stringArrayType(), value: func(ctx Context) object.Object { return stringArrayValue(ctx.Build.Creator.Teams) }},
	{name: "build.creator.verified", typ: boolType(), value: func(ctx Context) object.Object { return boolValue(ctx.Build.Creator.Verified) }},
	{name: "build.author.id", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.Author.ID) }},
	{name: "build.author.name", typ: stringType(), value: func(ctx Context) object.Object { return presenceStringValue(ctx.Build.Author.Name) }},
	{name: "build.author.email", typ: stringType(), value: func(ctx Context) object.Object { return presenceStringValue(ctx.Build.Author.Email) }},
	{name: "build.author.teams", typ: stringArrayType(), value: func(ctx Context) object.Object { return stringArrayValue(ctx.Build.Author.Teams) }},
	{name: "build.scm.author.name", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.SCM.AuthorName) }},
	{name: "build.scm.author.email", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.SCM.AuthorEmail) }},
	{name: "build.scm.committer.name", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.SCM.CommitterName) }},
	{name: "build.scm.committer.email", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.SCM.CommitterEmail) }},
	{name: "build.pull_request.id", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.PullRequest.ID) }},
	{name: "build.pull_request.base_branch", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.PullRequest.BaseBranch) }},
	{name: "build.pull_request.draft", typ: boolType(), value: func(ctx Context) object.Object { return boolValue(ctx.Build.PullRequest.Draft) }},
	{name: "build.pull_request.label", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(pullRequestLabel(ctx)) }},
	{name: "build.pull_request.labels", typ: stringArrayType(), value: func(ctx Context) object.Object { return stringArrayValue(ctx.Build.PullRequest.Labels) }},
	{name: "build.pull_request.repository", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.PullRequest.Repository) }},
	{name: "build.pull_request.repository.fork", typ: boolType(), value: func(ctx Context) object.Object { return boolValue(ctx.Build.PullRequest.RepositoryFork) }},
	{name: "build.merge_queue.base_branch", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.MergeQueue.BaseBranch) }},
	{name: "build.merge_queue.base_commit", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Build.MergeQueue.BaseCommit) }},
	{name: "pipeline.id", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Pipeline.ID) }},
	{name: "pipeline.slug", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Pipeline.Slug) }},
	{name: "pipeline.default_branch", typ: stringType(), value: func(ctx Context) object.Object { return presenceStringValue(ctx.Pipeline.DefaultBranch) }},
	{name: "pipeline.repository", typ: stringType(), value: func(ctx Context) object.Object { return presenceStringValue(ctx.Pipeline.Repository) }},
	// Upstream Build::Condition exposes these in its base assignment table,
	// even though the public docs describe them as notification variables.
	{name: "pipeline.started_passing", typ: boolType(), value: func(ctx Context) object.Object { return boolValue(ctx.Pipeline.StartedPassing) }},
	{name: "pipeline.started_failing", typ: boolType(), value: func(ctx Context) object.Object { return boolValue(ctx.Pipeline.StartedFailing) }},
	{name: "pipeline.next_finished_build_exists", typ: boolType(), value: func(ctx Context) object.Object { return boolValue(ctx.Pipeline.NextFinishedBuildExists) }},
	{name: "organization.id", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Organization.ID) }},
	{name: "organization.slug", typ: stringType(), value: func(ctx Context) object.Object { return stringValue(ctx.Organization.Slug) }},
}

var stepAssignmentDefinitions = []assignmentDefinition{
	{name: "step.id", typ: stringType(), value: func(ctx Context) object.Object {
		return stringValue(stepString(ctx.Step, func(step *Step) *string { return step.ID }))
	}},
	{name: "step.key", typ: stringType(), value: func(ctx Context) object.Object {
		return stringValue(stepString(ctx.Step, func(step *Step) *string { return step.Key }))
	}},
	{name: "step.type", typ: enumValueType("step type", "command", "wait", "input", "trigger", "group"), value: func(ctx Context) object.Object {
		return stringValue(stepString(ctx.Step, func(step *Step) *string { return step.Type }))
	}},
	{name: "step.label", typ: stringType(), value: func(ctx Context) object.Object {
		return stringValue(stepString(ctx.Step, func(step *Step) *string { return step.Label }))
	}},
	{name: "step.state", typ: enumValueType("step state", "ignored", "waiting_for_dependencies", "ready", "waiting_for_input", "running", "failing", "canceled", "finished"), value: func(ctx Context) object.Object {
		return stringValue(stepString(ctx.Step, func(step *Step) *string { return step.State }))
	}},
	{name: "step.outcome", typ: enumValueType("step outcome", "neutral", "passed", "soft_failed", "hard_failed", "errored"), value: func(ctx Context) object.Object {
		return stringValue(stepString(ctx.Step, func(step *Step) *string { return step.Outcome }))
	}},
}

func assignmentDefinitions(ctx Context) []assignmentDefinition {
	if !stepAllowed(ctx.EntryPoint) {
		return baseAssignmentDefinitions
	}

	definitions := make([]assignmentDefinition, 0, len(baseAssignmentDefinitions)+len(stepAssignmentDefinitions))
	definitions = append(definitions, baseAssignmentDefinitions...)
	return append(definitions, stepAssignmentDefinitions...)
}

func flatAssignments(ctx Context) object.Struct {
	definitions := assignmentDefinitions(ctx)
	assignments := make(object.Struct, len(definitions))
	for _, definition := range definitions {
		assignments[definition.name] = definition.value(ctx)
	}
	return assignments
}
