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

## Design

The root package is the public Buildkite API:

* `conditional.Validate` checks an expression for a Buildkite context.
* `conditional.Evaluate` evaluates an expression and returns a boolean result.
* `conditional.Context` defines the Buildkite entry point and available values.

The lexer, parser, and evaluator packages are implementation details derived
from [Writing an Interpreter in Go](https://interpreterbook.com).
