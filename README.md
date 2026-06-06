# Buildkite Conditional Evaluator

A small Go library for validating and evaluating Buildkite conditional
expressions.

## What's supported?

* Comparators: `== != =~ !~`
* Logical operators: `|| &&`
* Integers `12345`
* Strings `'foobar' or "foobar"`
* Booleans and nulls `true false null`
* Parenthesis to control order of evaluation `( )`
* Buildkite identifiers such as `build.branch`
* Regular expressions `/^v1\.0/`
* Function calls such as `env("FOO")` and `build.env("FOO")`
* Prefixes: `!`
* Arrays: `["foo","bar"] includes "foo"`

### Syntax Examples

```c
// individual terms
true
false
null

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

// complex expressions
((build.tag =~ /^v/) || (build.branch == "main"))

// array operations
["master","staging"] includes build.branch
```

The evaluator expects conditionals after Buildkite interpolation has already
run. In pipeline YAML, escape `$` anchors to avoid interpolation; by the time the
conditional is parsed, an end anchor should be a raw `$`. Regex escapes such as
`\$` are preserved as literal-dollar matches.

## Entrypoints

Set `Context.EntryPoint` to the Buildkite location where the conditional runs:

* `EntryPointBuildCondition` evaluates build conditionals without `step.*`.
* `EntryPointBuildConditionWithStep` evaluates build conditionals where step
  variables are available.
* `EntryPointBuildNotification` evaluates build notification conditionals.
  `Evaluate` converts parse, validation, and evaluation errors to `false`.
* `EntryPointStepNotification` evaluates step notification conditionals with
  `step.*` variables. `Evaluate` converts parse, validation, and evaluation
  errors to `false`.

## Variables

The root API builds flat Buildkite assignments from `Context`:

* `build.*` values come from `Context.Build`.
* `pipeline.*` values come from documented `Context.Pipeline` fields:
  `id`, `slug`, `default_branch`, `repository`, `started_passing`,
  `started_failing`, and `next_finished_build_exists`.
* `organization.*` values come from `Context.Organization`.
* `step.*` values come from `Context.Step` only for step-aware entrypoints.
* `env("NAME")` reads the merged build and project environment, returning an
  empty string when the name is absent.
* `build.env("NAME")` reads the same merged environment, returning `null` when
  the name is absent.

Missing documented nullable values evaluate as `null`. Unknown variables,
unknown functions, unsupported `BUILDKITE_*` env names, invalid regular
expressions, unsupported regular expression features, type mismatches, and
non-boolean final results fail closed.

`Validate` always reports parse and validation errors. `Evaluate` reports errors
for build condition entrypoints, while notification entrypoints return `false`
for parse, validation, and evaluation errors.

## Usage

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

## Conformance oracle

Committed conformance cases can be checked locally with:

```sh
mise run conformance:check
```

To compare those cases with a server-backed oracle, set
`CONDITIONAL_ORACLE_COMMAND` or pass `--oracle-command`:

```sh
go run ./cmd/conditional conformance --oracle-command ./server-oracle
```

The command sends one JSON request on stdin for each case. The oracle should
write a JSON response such as `{"result":true}` or `{"error_kind":"parse"}`.
Use `go run ./cmd/conditional conformance --list` to inspect the request shape.

## Design

The root package is the public Buildkite API:

* `conditional.Validate` checks an expression for a Buildkite context.
* `conditional.Evaluate` evaluates an expression and returns a boolean result.
* `conditional.Context` defines the Buildkite entry point and available values.

The internal lexer, parser, and evaluator packages are derived from
[Writing an Interpreter in Go](https://interpreterbook.com).
