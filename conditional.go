package conditional

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/buildkite/conditional/ast"
	"github.com/buildkite/conditional/evaluator"
	"github.com/buildkite/conditional/lexer"
	"github.com/buildkite/conditional/object"
	"github.com/buildkite/conditional/parser"
)

// Validate parses expression for the selected Buildkite context.
func Validate(expression string, ctx Context) error {
	entryPoint, err := normalizeEntryPoint(ctx.EntryPoint)
	if err != nil {
		return err
	}
	if strings.TrimSpace(expression) == "" {
		return nil
	}

	expr, err := parse(expression)
	if err != nil {
		return err
	}
	ctx.EntryPoint = entryPoint
	return validateExpression(expr, ctx)
}

// Evaluate evaluates expression in the selected Buildkite context.
func Evaluate(expression string, ctx Context) (bool, error) {
	entryPoint, err := normalizeEntryPoint(ctx.EntryPoint)
	if err != nil {
		return false, err
	}
	ctx.EntryPoint = entryPoint

	if strings.TrimSpace(expression) == "" && isNotificationEntryPoint(entryPoint) {
		return true, nil
	}

	result, err := evaluate(expression, ctx)
	if err != nil && isNotificationEntryPoint(entryPoint) {
		return false, nil
	}
	return result, err
}

func evaluate(expression string, ctx Context) (bool, error) {
	expr, err := parse(expression)
	if err != nil {
		return false, err
	}
	if err := validateExpression(expr, ctx); err != nil {
		return false, err
	}

	result := evaluator.Eval(expr, buildScope(ctx))
	switch result := result.(type) {
	case *object.Boolean:
		return result.Value, nil
	case *object.Null:
		return false, nil
	case *object.Error:
		return false, &Error{Kind: ErrorKindEvaluation, Message: result.Message}
	default:
		return false, &Error{
			Kind:    ErrorKindResult,
			Message: fmt.Sprintf("expected boolean result, got %s", result.Type()),
		}
	}
}

func parse(expression string) (ast.Expression, error) {
	l := lexer.New(expression)
	p := parser.New(l)
	expr := p.Parse()

	if errs := p.Errors(); len(errs) > 0 {
		return nil, &Error{Kind: ErrorKindParse, Message: strings.Join(errs, "; ")}
	}
	if expr == nil {
		return nil, &Error{Kind: ErrorKindParse, Message: "empty expression"}
	}

	return expr, nil
}

func validateExpression(expr ast.Expression, ctx Context) error {
	entryPoint := ctx.EntryPoint
	if !stepAllowed(entryPoint) && referencesRoot(expr, "step") {
		return &Error{
			Kind:    ErrorKindValidation,
			Message: fmt.Sprintf("step variables are not available for entry point %q", entryPoint),
		}
	}
	if err := validateEnvCalls(expr); err != nil {
		return err
	}
	if err := validateOperators(expr); err != nil {
		return err
	}
	return typeCheckExpression(expr, ctx)
}

func validateEnvCalls(expr ast.Expression) error {
	switch expr := expr.(type) {
	case *ast.PrefixExpression:
		return validateEnvCalls(expr.Right)
	case *ast.ConditionalExpression:
		if err := validateEnvCalls(expr.Condition); err != nil {
			return err
		}
		if err := validateEnvCalls(expr.Consequence); err != nil {
			return err
		}
		return validateEnvCalls(expr.Alternative)
	case *ast.InfixExpression:
		if err := validateEnvCalls(expr.Left); err != nil {
			return err
		}
		return validateEnvCalls(expr.Right)
	case *ast.CallExpression:
		if expr.Function == "env" || expr.Function == "build.env" {
			if len(expr.Arguments) != 1 {
				return &Error{
					Kind:    ErrorKindValidation,
					Message: fmt.Sprintf("%s expects exactly one argument", expr.Function),
				}
			}
			if arg, ok := expr.Arguments[0].(*ast.StringLiteral); ok && !runtimeStringLiteral(arg) {
				switch {
				case strings.HasPrefix(arg.Value, "$"):
					return &Error{
						Kind:    ErrorKindValidation,
						Message: "env argument should not include $",
					}
				case !validEnvName(arg.Value):
					return &Error{
						Kind:    ErrorKindValidation,
						Message: "env argument should be an environment variable name",
					}
				case unsupportedBuildkiteEnv(arg.Value):
					return &Error{
						Kind:    ErrorKindValidation,
						Message: fmt.Sprintf("%q is not a supported Buildkite environment variable", arg.Value),
					}
				}
			}
		}
		for _, arg := range expr.Arguments {
			if err := validateEnvCalls(arg); err != nil {
				return err
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if err := validateEnvCalls(element); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateOperators(expr ast.Expression) error {
	switch expr := expr.(type) {
	case *ast.PrefixExpression:
		return validateOperators(expr.Right)
	case *ast.ConditionalExpression:
		if err := validateOperators(expr.Condition); err != nil {
			return err
		}
		if err := validateOperators(expr.Consequence); err != nil {
			return err
		}
		return validateOperators(expr.Alternative)
	case *ast.InfixExpression:
		if expr.Operator == "@>" {
			return &Error{Kind: ErrorKindValidation, Message: "@> is not Buildkite conditional syntax"}
		}
		if err := validateOperators(expr.Left); err != nil {
			return err
		}
		return validateOperators(expr.Right)
	case *ast.CallExpression:
		for _, arg := range expr.Arguments {
			if err := validateOperators(arg); err != nil {
				return err
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if err := validateOperators(element); err != nil {
				return err
			}
		}
	}

	return nil
}

func referencesRoot(expr ast.Expression, root string) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value == root || strings.HasPrefix(expr.Value, root+".")
	case *ast.PrefixExpression:
		return referencesRoot(expr.Right, root)
	case *ast.ConditionalExpression:
		return referencesRoot(expr.Condition, root) ||
			referencesRoot(expr.Consequence, root) ||
			referencesRoot(expr.Alternative, root)
	case *ast.InfixExpression:
		return referencesRoot(expr.Left, root) || referencesRoot(expr.Right, root)
	case *ast.CallExpression:
		if expr.Function == root || strings.HasPrefix(expr.Function, root+".") {
			return true
		}
		for _, arg := range expr.Arguments {
			if referencesRoot(arg, root) {
				return true
			}
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if referencesRoot(element, root) {
				return true
			}
		}
	}

	return false
}

type evaluationScope struct {
	object.Struct
	env map[string]string
}

func (s evaluationScope) LookupEnv(key string) (string, bool) {
	value, ok := s.env[key]
	return value, ok
}

func buildScope(ctx Context) evaluationScope {
	env := mergedEnv(ctx)

	scope := object.Struct{
		"env":          envFunction(env),
		"build.env":    nullableEnvFunction(env),
		"build":        buildObject(ctx),
		"pipeline":     pipelineObject(ctx.Pipeline),
		"organization": organizationObject(ctx.Organization),
	}
	for key, value := range flatAssignments(ctx) {
		scope[key] = value
	}

	if stepAllowed(ctx.EntryPoint) {
		scope["step"] = stepObject(ctx.Step)
	}

	return evaluationScope{Struct: scope, env: env}
}

func envFunction(env map[string]string) object.Function {
	return func(args []object.Object) object.Object {
		name, err := envNameArg(args)
		if err != nil {
			return err
		}
		return &object.String{Value: env[name]}
	}
}

func nullableEnvFunction(env map[string]string) object.Function {
	return func(args []object.Object) object.Object {
		name, err := envNameArg(args)
		if err != nil {
			return err
		}
		value, ok := env[name]
		if !ok {
			return &object.Null{}
		}
		return &object.String{Value: value}
	}
}

func envNameArg(args []object.Object) (string, *object.Error) {
	if len(args) != 1 {
		return "", &object.Error{Message: fmt.Sprintf("wrong number of arguments for env: got %d, want 1", len(args))}
	}
	name, ok := args[0].(*object.String)
	if !ok {
		return "", &object.Error{Message: "env argument must be a string"}
	}
	if name.Value == "" {
		return "", &object.Error{Message: "env argument should be an environment variable name"}
	}
	if unsupportedBuildkiteEnv(name.Value) {
		return "", &object.Error{Message: fmt.Sprintf("interpolation of %q is not supported", name.Value)}
	}
	return name.Value, nil
}

func flatAssignments(ctx Context) object.Struct {
	build := ctx.Build
	assignments := object.Struct{
		"build.id":                           stringValue(build.ID),
		"build.state":                        stringValue(build.State),
		"build.fixed":                        boolValue(build.Fixed),
		"build.blocked_state":                stringValue(build.BlockedState),
		"build.source":                       stringValue(build.Source),
		"build.source_event":                 stringValue(sourceEvent(ctx)),
		"build.source_action":                stringValue(sourceAction(ctx)),
		"build.branch":                       stringValue(build.Branch),
		"build.tag":                          stringValue(build.Tag),
		"build.message":                      stringValue(build.Message),
		"build.commit":                       stringValue(build.Commit),
		"build.number":                       intValue(build.Number),
		"build.creator.id":                   stringValue(build.Creator.ID),
		"build.creator.name":                 stringValue(build.Creator.Name),
		"build.creator.email":                stringValue(build.Creator.Email),
		"build.creator.teams":                stringArrayValue(build.Creator.Teams),
		"build.creator.verified":             boolValue(build.Creator.Verified),
		"build.author.id":                    stringValue(build.Author.ID),
		"build.author.name":                  stringValue(build.Author.Name),
		"build.author.email":                 stringValue(build.Author.Email),
		"build.author.teams":                 stringArrayValue(build.Author.Teams),
		"build.scm.author.name":              stringValue(build.SCM.AuthorName),
		"build.scm.author.email":             stringValue(build.SCM.AuthorEmail),
		"build.scm.committer.name":           stringValue(build.SCM.CommitterName),
		"build.scm.committer.email":          stringValue(build.SCM.CommitterEmail),
		"build.pull_request.id":              stringValue(build.PullRequest.ID),
		"build.pull_request.base_branch":     stringValue(build.PullRequest.BaseBranch),
		"build.pull_request.draft":           boolValue(build.PullRequest.Draft),
		"build.pull_request.label":           stringValue(pullRequestLabel(ctx)),
		"build.pull_request.labels":          stringArrayValue(build.PullRequest.Labels),
		"build.pull_request.repository":      stringValue(build.PullRequest.Repository),
		"build.pull_request.repository.fork": boolValue(build.PullRequest.RepositoryFork),
		"build.merge_queue.base_branch":      stringValue(build.MergeQueue.BaseBranch),
		"build.merge_queue.base_commit":      stringValue(build.MergeQueue.BaseCommit),
		"pipeline.id":                        stringValue(ctx.Pipeline.ID),
		"pipeline.name":                      stringValue(ctx.Pipeline.Name),
		"pipeline.slug":                      stringValue(ctx.Pipeline.Slug),
		"pipeline.default_branch":            stringValue(ctx.Pipeline.DefaultBranch),
		"pipeline.repository":                stringValue(ctx.Pipeline.Repository),
		// Upstream Build::Condition exposes these in its base assignment table,
		// even though the public docs describe them as notification variables.
		"pipeline.started_passing":            boolValue(ctx.Pipeline.StartedPassing),
		"pipeline.started_failing":            boolValue(ctx.Pipeline.StartedFailing),
		"pipeline.next_finished_build_exists": boolValue(ctx.Pipeline.NextFinishedBuildExists),
		"organization.id":                     stringValue(ctx.Organization.ID),
		"organization.slug":                   stringValue(ctx.Organization.Slug),
	}

	if stepAllowed(ctx.EntryPoint) {
		step := stepObject(ctx.Step)
		assignments["step.id"] = step["id"]
		assignments["step.key"] = step["key"]
		assignments["step.type"] = step["type"]
		assignments["step.label"] = step["label"]
		assignments["step.state"] = step["state"]
		assignments["step.outcome"] = step["outcome"]
	}

	return assignments
}

func buildObject(ctx Context) object.Struct {
	build := ctx.Build
	return object.Struct{
		"id":            stringValue(build.ID),
		"state":         stringValue(build.State),
		"fixed":         boolValue(build.Fixed),
		"blocked_state": stringValue(build.BlockedState),
		"source":        stringValue(build.Source),
		"source_event":  stringValue(sourceEvent(ctx)),
		"source_action": stringValue(sourceAction(ctx)),
		"branch":        stringValue(build.Branch),
		"tag":           stringValue(build.Tag),
		"message":       stringValue(build.Message),
		"commit":        stringValue(build.Commit),
		"number":        intValue(build.Number),
		"creator":       actorObject(build.Creator, true),
		"author":        actorObject(build.Author, false),
		"scm": object.Struct{
			"author": object.Struct{
				"name":  stringValue(build.SCM.AuthorName),
				"email": stringValue(build.SCM.AuthorEmail),
			},
			"committer": object.Struct{
				"name":  stringValue(build.SCM.CommitterName),
				"email": stringValue(build.SCM.CommitterEmail),
			},
		},
		"pull_request": object.Struct{
			"id":          stringValue(build.PullRequest.ID),
			"base_branch": stringValue(build.PullRequest.BaseBranch),
			"draft":       boolValue(build.PullRequest.Draft),
			"label":       stringValue(pullRequestLabel(ctx)),
			"labels":      stringArrayValue(build.PullRequest.Labels),
			"repository": object.Struct{
				"fork": boolValue(build.PullRequest.RepositoryFork),
			},
		},
		"merge_queue": object.Struct{
			"base_branch": stringValue(build.MergeQueue.BaseBranch),
			"base_commit": stringValue(build.MergeQueue.BaseCommit),
		},
	}
}

func actorObject(actor Actor, includeVerified bool) object.Struct {
	out := object.Struct{
		"id":    stringValue(actor.ID),
		"name":  stringValue(actor.Name),
		"email": stringValue(actor.Email),
		"teams": stringArrayValue(actor.Teams),
	}
	if includeVerified {
		out["verified"] = boolValue(actor.Verified)
	}
	return out
}

func pipelineObject(pipeline Pipeline) object.Struct {
	return object.Struct{
		"id":                         stringValue(pipeline.ID),
		"name":                       stringValue(pipeline.Name),
		"slug":                       stringValue(pipeline.Slug),
		"default_branch":             stringValue(pipeline.DefaultBranch),
		"repository":                 stringValue(pipeline.Repository),
		"started_passing":            boolValue(pipeline.StartedPassing),
		"started_failing":            boolValue(pipeline.StartedFailing),
		"next_finished_build_exists": boolValue(pipeline.NextFinishedBuildExists),
	}
}

func organizationObject(organization Organization) object.Struct {
	return object.Struct{
		"id":   stringValue(organization.ID),
		"slug": stringValue(organization.Slug),
	}
}

func stepObject(step *Step) object.Struct {
	if step == nil {
		return object.Struct{
			"id":      &object.Null{},
			"key":     &object.Null{},
			"type":    &object.Null{},
			"label":   &object.Null{},
			"state":   &object.Null{},
			"outcome": &object.Null{},
		}
	}
	return object.Struct{
		"id":      stringValue(step.ID),
		"key":     stringValue(step.Key),
		"type":    stringValue(step.Type),
		"label":   stringValue(step.Label),
		"state":   stringValue(step.State),
		"outcome": stringValue(step.Outcome),
	}
}

func sourceEvent(ctx Context) *string {
	if stringPtrValue(ctx.Build.Source) != "webhook" {
		return nil
	}
	if ctx.Build.SourceEvent != nil {
		return ctx.Build.SourceEvent
	}
	if value, ok := ctx.BuildEnv["BUILDKITE_GITHUB_EVENT"]; ok {
		return &value
	}
	return nil
}

func sourceAction(ctx Context) *string {
	if stringPtrValue(ctx.Build.Source) != "webhook" {
		return nil
	}
	if ctx.Build.SourceAction != nil {
		return ctx.Build.SourceAction
	}
	if value, ok := ctx.BuildEnv["BUILDKITE_GITHUB_ACTION"]; ok {
		return &value
	}
	return nil
}

func pullRequestLabel(ctx Context) *string {
	event := sourceEvent(ctx)
	if event == nil || *event != "pull_request" {
		return nil
	}
	action := sourceAction(ctx)
	if action == nil || (*action != "labeled" && *action != "unlabeled") {
		return nil
	}
	return ctx.Build.PullRequest.Label
}

var supportedBuildkiteEnv = map[string]struct{}{
	"BUILDKITE_BRANCH":                               {},
	"BUILDKITE_TAG":                                  {},
	"BUILDKITE_MESSAGE":                              {},
	"BUILDKITE_COMMIT":                               {},
	"BUILDKITE_PIPELINE_SLUG":                        {},
	"BUILDKITE_PIPELINE_NAME":                        {},
	"BUILDKITE_PIPELINE_ID":                          {},
	"BUILDKITE_ORGANIZATION_SLUG":                    {},
	"BUILDKITE_TRIGGERED_FROM_BUILD_ID":              {},
	"BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER":          {},
	"BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG":   {},
	"BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID":          {},
	"BUILDKITE_REBUILT_FROM_BUILD_ID":                {},
	"BUILDKITE_REBUILT_FROM_BUILD_NUMBER":            {},
	"BUILDKITE_REPO":                                 {},
	"BUILDKITE_PULL_REQUEST":                         {},
	"BUILDKITE_PULL_REQUEST_BASE_BRANCH":             {},
	"BUILDKITE_PULL_REQUEST_REPO":                    {},
	"BUILDKITE_PULL_REQUEST_LABELS":                  {},
	"BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC":     {},
	"BUILDKITE_MERGE_QUEUE_BASE_BRANCH":              {},
	"BUILDKITE_MERGE_QUEUE_BASE_COMMIT":              {},
	"BUILDKITE_GIT_DIFF_BASE":                        {},
	"BUILDKITE_GITHUB_ACTION":                        {},
	"BUILDKITE_GITHUB_COMMENT_ID":                    {},
	"BUILDKITE_GITHUB_DEPLOYMENT_ID":                 {},
	"BUILDKITE_GITHUB_DEPLOYMENT_TASK":               {},
	"BUILDKITE_GITHUB_DEPLOYMENT_ENVIRONMENT":        {},
	"BUILDKITE_GITHUB_DEPLOYMENT_PAYLOAD":            {},
	"BUILDKITE_GITHUB_EVENT":                         {},
	"BUILDKITE_GITHUB_REVIEW_ID":                     {},
	"BUILDKITE_GITHUB_CHECK_RUN_CONCLUSION":          {},
	"BUILDKITE_GITHUB_CHECK_RUN_NAME":                {},
	"BUILDKITE_GITHUB_DEPLOYMENT_STATUS_ENVIRONMENT": {},
	"BUILDKITE_GITHUB_DEPLOYMENT_STATUS_STATE":       {},
	"BUILDKITE_GITHUB_RELEASE_DRAFT":                 {},
	"BUILDKITE_GITHUB_RELEASE_PRERELEASE":            {},
	"BUILDKITE_GITHUB_RELEASE_TAG":                   {},
	"BUILDKITE_GITHUB_REVIEW_STATE":                  {},
}

func mergedEnv(ctx Context) map[string]string {
	env := make(map[string]string, len(ctx.ProjectEnv)+len(ctx.BuildEnv)+len(supportedBuildkiteEnv))
	mergeUserEnv(env, ctx.ProjectEnv)
	mergeUserEnv(env, ctx.BuildEnv)
	for key, value := range builtinEnv(ctx) {
		env[key] = value
	}
	return env
}

func mergeUserEnv(env map[string]string, values map[string]string) {
	for key, value := range values {
		if unsupportedBuildkiteEnv(key) {
			continue
		}
		env[key] = value
	}
}

func unsupportedBuildkiteEnv(key string) bool {
	if !strings.HasPrefix(key, "BUILDKITE_") {
		return false
	}
	_, ok := supportedBuildkiteEnv[key]
	return !ok
}

func validEnvName(key string) bool {
	if key == "" {
		return false
	}
	for _, ch := range key {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		return false
	}
	return true
}

func builtinEnv(ctx Context) map[string]string {
	env := map[string]string{}

	setString(env, "BUILDKITE_BRANCH", ctx.Build.Branch)
	setString(env, "BUILDKITE_TAG", ctx.Build.Tag)
	if ctx.Build.Tag == nil {
		env["BUILDKITE_TAG"] = ""
	}
	setString(env, "BUILDKITE_MESSAGE", ctx.Build.Message)
	if ctx.Build.Message == nil {
		env["BUILDKITE_MESSAGE"] = ""
	}
	setString(env, "BUILDKITE_COMMIT", ctx.Build.Commit)
	setString(env, "BUILDKITE_REPO", ctx.Pipeline.Repository)
	setString(env, "BUILDKITE_PIPELINE_SLUG", ctx.Pipeline.Slug)
	setString(env, "BUILDKITE_PIPELINE_NAME", ctx.Pipeline.Name)
	setString(env, "BUILDKITE_PIPELINE_ID", ctx.Pipeline.ID)
	setString(env, "BUILDKITE_ORGANIZATION_SLUG", ctx.Organization.Slug)

	if ctx.Build.PullRequest.ID == nil || *ctx.Build.PullRequest.ID == "" {
		env["BUILDKITE_PULL_REQUEST"] = "false"
	} else {
		env["BUILDKITE_PULL_REQUEST"] = *ctx.Build.PullRequest.ID
	}
	setStringOrBlank(env, "BUILDKITE_PULL_REQUEST_BASE_BRANCH", ctx.Build.PullRequest.BaseBranch)
	setStringOrBlank(env, "BUILDKITE_PULL_REQUEST_REPO", ctx.Build.PullRequest.Repository)
	env["BUILDKITE_PULL_REQUEST_LABELS"] = strings.Join(ctx.Build.PullRequest.Labels, ",")
	if boolPtrValue(ctx.Build.PullRequest.UsingMergeRefspec) {
		env["BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC"] = "true"
	} else {
		env["BUILDKITE_PULL_REQUEST_USING_MERGE_REFSPEC"] = ""
	}
	setStringOrBlank(env, "BUILDKITE_MERGE_QUEUE_BASE_BRANCH", ctx.Build.MergeQueue.BaseBranch)
	setStringOrBlank(env, "BUILDKITE_MERGE_QUEUE_BASE_COMMIT", ctx.Build.MergeQueue.BaseCommit)
	setStringOrBlank(env, "BUILDKITE_TRIGGERED_FROM_BUILD_ID", ctx.Build.TriggeredFrom.BuildID)
	setIntOrBlank(env, "BUILDKITE_TRIGGERED_FROM_BUILD_NUMBER", ctx.Build.TriggeredFrom.BuildNumber)
	setStringOrBlank(env, "BUILDKITE_TRIGGERED_FROM_BUILD_PIPELINE_SLUG", ctx.Build.TriggeredFrom.PipelineSlug)
	setStringOrBlank(env, "BUILDKITE_TRIGGERED_FROM_BUILD_JOB_ID", ctx.Build.TriggeredFrom.JobID)
	setStringOrBlank(env, "BUILDKITE_REBUILT_FROM_BUILD_ID", ctx.Build.RebuiltFrom.BuildID)
	setIntOrBlank(env, "BUILDKITE_REBUILT_FROM_BUILD_NUMBER", ctx.Build.RebuiltFrom.BuildNumber)
	env["BUILDKITE_GIT_DIFF_BASE"] = gitDiffBase(ctx)

	return env
}

func setString(env map[string]string, key string, value *string) {
	if value != nil {
		env[key] = *value
	}
}

func setStringOrBlank(env map[string]string, key string, value *string) {
	if value == nil {
		env[key] = ""
		return
	}
	env[key] = *value
}

func setIntOrBlank(env map[string]string, key string, value *int) {
	if value == nil {
		env[key] = ""
		return
	}
	env[key] = strconv.Itoa(*value)
}

func boolPtrValue(value *bool) bool {
	return value != nil && *value
}

func gitDiffBase(ctx Context) string {
	if !ctx.Build.MergeQueue.Active {
		return ""
	}
	if boolPtrValue(ctx.Pipeline.UseMergeQueueBaseCommitForGitDiffBase) {
		return stringPtrValue(ctx.Build.MergeQueue.BaseCommit)
	}
	return stringPtrValue(ctx.Build.MergeQueue.BaseBranch)
}

func normalizeEntryPoint(entryPoint EntryPoint) (EntryPoint, error) {
	switch entryPoint {
	case "", EntryPointBuildCondition:
		return EntryPointBuildCondition, nil
	case EntryPointBuildConditionWithStep, EntryPointBuildNotification, EntryPointStepNotification:
		return entryPoint, nil
	default:
		return "", &Error{
			Kind:    ErrorKindValidation,
			Message: fmt.Sprintf("unknown entry point %q", entryPoint),
		}
	}
}

func stepAllowed(entryPoint EntryPoint) bool {
	return entryPoint == EntryPointBuildConditionWithStep || entryPoint == EntryPointStepNotification
}

func isNotificationEntryPoint(entryPoint EntryPoint) bool {
	return entryPoint == EntryPointBuildNotification || entryPoint == EntryPointStepNotification
}

func stringValue(value *string) object.Object {
	if value == nil {
		return &object.Null{}
	}
	return &object.String{Value: *value}
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolValue(value *bool) object.Object {
	if value == nil {
		return &object.Null{}
	}
	return &object.Boolean{Value: *value}
}

func intValue(value *int) object.Object {
	if value == nil {
		return &object.Null{}
	}
	return &object.Integer{Value: int64(*value)}
}

func stringArrayValue(values []string) object.Object {
	if values == nil {
		return &object.Null{}
	}

	elements := make([]object.Object, 0, len(values))
	for _, value := range values {
		elements = append(elements, &object.String{Value: value})
	}
	return &object.Array{Elements: elements}
}
