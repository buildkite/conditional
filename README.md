# Buildkite Conditional Evaluator

A small c-like language for evaluating boolean conditions, used in Buildkite's pipeline.yml format and for filtering whether webhooks are accepted.

## What's supported?

* Comparators: `== != =~ !~`
* Logical operators: `|| &&`
* Integers `12345`
* Strings `'foobar' or "foobar"`
* Booleans and nulls `true false null`
* Parenthesis to control order of evaluation `( )`
* Object dereferencing `foo.bar`
* Regular expressions `/^v1\.0/`
* Function calls `foo("bar")`
* Prefixes: `!`
* Arrays: `["foo","bar"] includes "foo"` (`@>` is also supported for compatibility)

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
env('FOO') == "BAR"
env('FOO') == obj.bar
env(env('BAR')) == "FOO"

// regular expression matches
build.tag =~ /^v/
build.message !~ /\[skip tests\]/i

// complex expressions
((build.tag =~ ^v) || (meta-data("foo") == "bar"))

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

	"github.com/buildkite/conditional/evaluator"
	"github.com/buildkite/conditional/lexer"
	"github.com/buildkite/conditional/object"
	"github.com/buildkite/conditional/parser"
)

func main() {
	l := lexer.New(`build.message =~ /^llamas rock/`)
	p := parser.New(l)
	expr := p.Parse()

	if errs := p.Errors(); len(errs) > 0 {
		log.Fatal(errs)
	}

	obj := evaluator.Eval(expr, object.Struct{
		"build": object.Struct{
			"message": &object.String{"llamas rock, and so do alpacas"},
		},
	})

	log.Printf("Result: %#v", obj)
}
```

## Design

Largely derived from [Writing an Interpreter in Go](https://interpreterbook.com):

* `lexer.Lexer` takes a string of input and turns it into a stream of `token.Token`
* `parser.Parser` takes a Lexer and parses tokens into an `ast.Expression`
* `evaluator.Evaluator` which takes a `ast.Expression` and evaluates it, with a `object.Map` for variables in scope. An `*object.Object` is returned.
