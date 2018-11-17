# Buildkite Condition Evaluator

A small c-like language for evaluating boolean conditions, used in Buildkite's pipeline.yml format and for filtering whether webhooks are accepted.

## What's supported?

* Comparators: `== != =~ !~`
* Logical operators: `|| &&`
* Integers `12345`
* Strings `'foobar' or "foobar"`
* Booleans `true false`
* Parenthesis to control order of evaluation `( )`
* Object dereferencing `foo.bar`
* Regular expressions `/^v1\.0/`
* Function calls `foo("bar")`
* Prefixes: `!`

## Syntax:

```c
// individual terms
true
false

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

// complex expressions
((build.tag =~ ^v) || (meta-data("foo") == "bar"))
```
