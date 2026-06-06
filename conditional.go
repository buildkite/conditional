package conditional

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/buildkite/conditional/internal/ast"
	"github.com/buildkite/conditional/internal/evaluator"
	"github.com/buildkite/conditional/internal/lexer"
	"github.com/buildkite/conditional/internal/object"
	"github.com/buildkite/conditional/internal/parser"
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
						Message: envDollarNameMessage(arg.Value),
					}
				case !validEnvName(arg.Value):
					return &Error{
						Kind:    ErrorKindValidation,
						Message: "Argument to `env` should be an environment variable name",
					}
				case unsupportedBuildkiteEnv(arg.Value):
					if suggestion := suggestBuildkiteEnv(arg.Value); suggestion != "" {
						return &Error{
							Kind: ErrorKindValidation,
							Message: fmt.Sprintf(
								"%q is not a valid environment variable - did you mean %q?",
								arg.Value,
								suggestion,
							),
						}
					}
					return &Error{
						Kind:    ErrorKindValidation,
						Message: unsupportedBuildkiteEnvMessage(arg.Value),
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
		"env":       envFunction(env),
		"build.env": nullableEnvFunction(env),
	}
	for key, value := range flatAssignments(ctx) {
		scope[key] = value
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
	if unsupportedRuntimeBuildkiteEnv(name.Value) {
		return "", &object.Error{Message: unsupportedBuildkiteEnvMessage(name.Value)}
	}
	return name.Value, nil
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

func presenceStringValue(value *string) object.Object {
	if value == nil || strings.TrimSpace(*value) == "" {
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
