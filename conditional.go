package conditional

import (
	"fmt"
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
	return validateExpression(expr, entryPoint)
}

// Evaluate evaluates expression in the selected Buildkite context.
func Evaluate(expression string, ctx Context) (bool, error) {
	entryPoint, err := normalizeEntryPoint(ctx.EntryPoint)
	if err != nil {
		return false, err
	}
	ctx.EntryPoint = entryPoint

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
	if err := validateExpression(expr, ctx.EntryPoint); err != nil {
		return false, err
	}

	result := evaluator.Eval(expr, buildScope(ctx))
	switch result := result.(type) {
	case *object.Boolean:
		return result.Value, nil
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

func validateExpression(expr ast.Expression, entryPoint EntryPoint) error {
	if !stepAllowed(entryPoint) && referencesRoot(expr, "step") {
		return &Error{
			Kind:    ErrorKindValidation,
			Message: fmt.Sprintf("step variables are not available for entry point %q", entryPoint),
		}
	}
	return nil
}

func referencesRoot(expr ast.Expression, root string) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value == root
	case *ast.PrefixExpression:
		return referencesRoot(expr.Right, root)
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

func buildScope(ctx Context) object.Struct {
	env := mergedEnv(ctx)

	scope := object.Struct{
		"env": object.Function(func(args []object.Object) object.Object {
			if len(args) != 1 {
				return &object.Error{Message: fmt.Sprintf("wrong number of arguments for env: got %d, want 1", len(args))}
			}
			name, ok := args[0].(*object.String)
			if !ok {
				return &object.Error{Message: "env argument must be a string"}
			}
			return &object.String{Value: env[name.Value]}
		}),
		"build":        buildObject(ctx),
		"pipeline":     pipelineObject(ctx.Pipeline),
		"organization": organizationObject(ctx.Organization),
	}

	if stepAllowed(ctx.EntryPoint) {
		scope["step"] = stepObject(ctx.Step)
	}

	return scope
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
	return ctx.Build.PullRequest.Label
}

func mergedEnv(ctx Context) map[string]string {
	env := make(map[string]string, len(ctx.ProjectEnv)+len(ctx.BuildEnv))
	for key, value := range ctx.ProjectEnv {
		env[key] = value
	}
	for key, value := range ctx.BuildEnv {
		env[key] = value
	}
	return env
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
