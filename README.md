# Buildkite Conditional Evaluator

[![Build status](https://badge.buildkite.com/e7ce8917b2bbb76ee5ee7a6d374b019ab56fad137ef05ee070.svg?branch=master)](https://buildkite.com/buildkite/conditional)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Go library for validating and evaluating Buildkite conditional expressions
with the same server-side syntax and semantics documented in
[Using conditionals](https://buildkite.com/docs/pipelines/configure/conditionals).

## Parity target

`conditional` is intended to answer the same yes/no question that Buildkite
answers when it evaluates a pipeline `if` attribute or notification condition.
The public API models the Buildkite server inputs through `Context`, then parses,
validates, and evaluates the expression against the documented conditional
language.

Any divergence from Buildkite's server-side conditional behavior should be
treated as a bug. The library does not parse pipeline YAML or run the full
pipeline upload process; callers provide the conditional expression and the
Buildkite values that would be available at the selected entrypoint.

## Supported syntax

* Comparators: `== != =~ !~`
* Logical operators: `|| &&`
* Ternary conditionals: `condition ? when_true : when_false`
* Parentheses to control grouping: `( )`
* Literals: integers, strings, booleans, `null`, arrays, and regular
  expressions
* Buildkite identifiers such as `build.branch`
* Function calls such as `env("FOO")` and `build.env("FOO")`; dotted function
  names are parsed as flat function names
* Prefix negation: `!`
* Array membership: `["foo", "bar"] includes "foo"`
* Shell-style environment substitution in operands and double-quoted strings
* `//` comments

### Syntax examples

```c
// individual terms
true
false
null
12345
"foobar"
'foobar'
["master", "staging"]

// compare values
build.branch == "master"
build.tag != "v1.0.0"
"blah" == 'blah'

// function calls
env("FOO") == "BAR"
build.env("BUILDKITE_BRANCH") == build.branch

// regular expression matches
build.tag =~ /^v/
build.message !~ /\[skip tests\]/i

// logical and ternary expressions
(build.tag =~ /^v/) || (build.branch == "main")
build.pull_request.id == null ? build.branch == "main" : true

// array operations
["master", "staging"] includes build.branch
build.creator.teams includes "deploy"

// shell-style substitutions
$branch == "main"
${branch:-main} == "main"
"deploy-${branch}" == "deploy-main"

// comments
build.branch == "main" // release branch
```

The evaluator parses the expression as the Buildkite server sees it. When an
expression is embedded in pipeline YAML, upload-time interpolation may run
before server-side conditional parsing, so escape `$` where the upload phase
should leave a literal dollar in the conditional. Inside conditional syntax,
shell-style substitutions such as `$branch` and `${branch:-main}` are evaluated
against the Buildkite environment, while regex escapes such as `\$` keep their
regular-expression meaning.

## Entrypoints

Set `Context.EntryPoint` to the Buildkite location where the conditional runs:

* `EntryPointBuildCondition` evaluates build conditionals without `step.*`.
  This is also the default when `Context.EntryPoint` is empty.
* `EntryPointBuildConditionWithStep` evaluates build conditionals where step
  variables are available.
* `EntryPointBuildNotification` evaluates build notification conditionals.
  `Evaluate` converts parse, validation, and evaluation errors to `false`.
* `EntryPointStepNotification` evaluates step notification conditionals with
  `step.*` variables. `Evaluate` converts parse, validation, and evaluation
  errors to `false`.

## Variables

The root API builds flat Buildkite assignments from `Context`, matching the
server's conditional assignment table:

* `build.*` values come from `Context.Build`.
* `pipeline.*` values come from documented `Context.Pipeline` fields:
  `id`, `slug`, `default_branch`, `repository`, `started_passing`,
  `started_failing`, and `next_finished_build_exists`.
* `organization.*` values come from `Context.Organization`.
* `step.*` values come from `Context.Step` only for step-aware entrypoints.

Missing documented nullable values evaluate as `null`. Unknown variables,
unknown functions, invalid regular expressions, and server-unsupported regular
expression features fail validation or parsing. Type mismatches, evaluation
errors, and non-boolean final results fail closed.

## Environment

`Context.ProjectEnv` and `Context.BuildEnv` provide caller-supplied
environment. Matching `Build::PipelineEnvironment`, project environment is
applied first, build environment overrides it, and built-in Buildkite values
derived from `Context` override both.

* `env("NAME")` reads the merged environment and returns a string. Missing
  values return `""`.
* `build.env("NAME")` reads the same merged environment. Missing values return
  `null`; present empty values return `""`.
* Shell substitutions read the same merged environment. An unset standalone
  substitution evaluates to `null`, and substitutions inside double-quoted
  strings follow the server's shell-style expansion rules.
* Literal `BUILDKITE_*` names passed to `env()` or `build.env()` are validated
  against the server's static supported environment allowlist.
* Dynamic `BUILDKITE_*` names are validated at runtime. This matches server
  behavior for values such as `BUILDKITE_PULL_REQUEST_LABELS`, which is
  runtime-derived but not accepted as a literal static `env()` or `build.env()`
  argument.

`Validate` always reports parse and validation errors. `Evaluate` reports errors
for build condition entrypoints, while notification entrypoints return `false`
for parse, validation, and evaluation errors. Blank notification conditionals
evaluate to `true`, matching Buildkite notification deliverability.

Returned errors are `*conditional.Error` values with a stable `Kind`. Parse
errors also unwrap to the underlying parser errors, so callers can inspect the
cause with `errors.Unwrap`.

## Extensions

`Validate` and `Evaluate` accept variadic options. With no options, the library
keeps the Buildkite server-parity contract and rejects unknown functions.
Callers can opt into additional functions for their own expression surface.
`Context` remains the per-call Buildkite state; options configure evaluator
capabilities.

Use per-call options when a custom function is only needed in one place:

```go
startsWith := conditional.WithFunction("starts_with", conditional.Function{
	Args:   []conditional.ValueType{conditional.StringType, conditional.StringType},
	Return: conditional.BoolType,
	Eval: func(args []conditional.Value) (conditional.Value, error) {
		value, _ := args[0].AsString()
		prefix, _ := args[1].AsString()
		return conditional.BoolValue(strings.HasPrefix(value, prefix)), nil
	},
})

ok, err := conditional.Evaluate(
	`starts_with(build.branch, "release/")`,
	ctx,
	startsWith,
)
```

Use `NewEvaluator` when the same options should be reused across many
validations or evaluations:

```go
evaluator, err := conditional.NewEvaluator(startsWith)
if err != nil {
	log.Fatal(err)
}

ok, err := evaluator.Evaluate(
	`starts_with(build.branch, "release/")`,
	ctx,
)
```

`NewEvaluator` validates options once. The zero value `Evaluator` has no custom
functions and behaves like the package-level Buildkite-parity helpers.

Custom functions are type-checked before evaluation. Function arguments and
return values use the exported `Value` and `ValueType` APIs, so callers do not
depend on internal evaluator objects. The `build`, `env`, `organization`,
`pipeline`, and `step` roots are reserved for Buildkite values and built-in
functions.

## Usage

Evaluate a build conditional:

```go
branch := "main"
message := "ship it"

ok, err := conditional.Evaluate(
	`build.branch == "main" && build.message !~ /\[skip tests\]/i`,
	conditional.Context{
		EntryPoint: conditional.EntryPointBuildCondition,
		Build: conditional.Build{
			Branch:  &branch,
			Message: &message,
		},
	},
)
if err != nil {
	log.Fatal(err)
}

log.Printf("should run: %t", ok)
```

Validate a conditional before storing it:

```go
err := conditional.Validate(
	`build.env("DEPLOY_ENV") == "production" && ${branch:-main} == "main"`,
	conditional.Context{EntryPoint: conditional.EntryPointBuildCondition},
)
if err != nil {
	log.Fatal(err)
}
```

Evaluate with Buildkite and custom environment values:

```go
branch := "main"

ok, err := conditional.Evaluate(
	`build.env("DEPLOY_ENV") == "production" && ${branch:-main} == "main"`,
	conditional.Context{
		EntryPoint: conditional.EntryPointBuildCondition,
		Build: conditional.Build{
			Branch: &branch,
		},
		ProjectEnv: map[string]string{
			"DEPLOY_ENV": "staging",
		},
		BuildEnv: map[string]string{
			"DEPLOY_ENV": "production",
			"branch":     "main",
		},
	},
)
if err != nil {
	log.Fatal(err)
}

log.Printf("should deploy: %t", ok)
```

Evaluate a step notification conditional:

```go
outcome := "passed"

deliver, err := conditional.Evaluate(
	`step.outcome == "passed"`,
	conditional.Context{
		EntryPoint: conditional.EntryPointStepNotification,
		Step: &conditional.Step{
			Outcome: &outcome,
		},
	},
)
if err != nil {
	log.Fatal(err)
}

log.Printf("should notify: %t", deliver)
```

For notification entrypoints, `Evaluate` returns `false` instead of surfacing
parse, validation, or evaluation errors, matching Buildkite notification
deliverability.

Full example:

```go
package main

import (
	"log"

	"github.com/buildkite/conditional"
)

func main() {
	message := "llamas rock, and so do alpacas"

	ok, err := conditional.Evaluate(`build.message =~ /^llamas rock/`, conditional.Context{
		EntryPoint: conditional.EntryPointBuildCondition,
		Build: conditional.Build{
			Message: &message,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Result: %#v", ok)
}
```

## Testing

Run the full local verification suite with:

```sh
mise run check
```

The local Go tests include source-tagged parity cases from the Buildkite docs
and upstream `buildkite/buildkite` specs. There is no live server comparison in
the default test path.

## Design

The root package is the public Buildkite API:

* `conditional.Validate` checks an expression for a Buildkite context.
* `conditional.Evaluate` evaluates an expression and returns a boolean result.
* `conditional.Context` defines the Buildkite entry point and available values.

The internal lexer, parser, and evaluator packages are derived from
[Writing an Interpreter in Go](https://interpreterbook.com).
