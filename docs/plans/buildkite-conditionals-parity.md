---
status: proposed
last_reviewed: 2026-06-06
spec_refs:
  - https://buildkite.com/docs/pipelines/configure/conditionals
---

# Buildkite Conditionals Parity

## Summary

This plan brings `buildkite/conditional` to exact parity with the server-side
Buildkite conditional language. The public docs remain the user-facing contract,
but the implementation contract is the server grammar, type checker, evaluator,
regular-expression validator, Buildkite context builder, and upstream specs in
`buildkite/buildkite`.

The current repository is now a modern Go module with `mise` tasks, CI wiring,
core boolean syntax, `null`, `includes`, short-circuiting logical operators, and
regexp2-based regular expressions. The remaining work is not just adding
operators. The library needs to become a polished Go package whose public API
evaluates Buildkite conditionals exactly as the server does, and whose internal
parser/evaluator packages are clean implementation details rather than a second
language with compatible-looking syntax.

The target outcome is a small, well-tested library that can answer the same
question as Buildkite: given a conditional expression, a Buildkite evaluation
context, and the source string form that Buildkite would evaluate, does the
expression evaluate to `true`, `false`, or a fail-closed error?

The hard part is parity evidence. The public docs list syntax, variables, and
examples, but some semantics are only observable from the server evaluator:
missing values, context-specific variables, type mismatches, dotted identifier
behavior, shell-style environment substitution, and server-rejected regular
expression features. The upstream `buildkite/buildkite` repo already has RSpec
coverage for much of this behavior. This plan therefore treats ported
table-driven Go conformance tests and an optional server oracle as first-class
deliverables, not test polish after implementation.

## Problem

The library currently evaluates a generic expression language with a generic
`object.Struct` scope. That is useful scaffolding, but it is not the same thing
as the server-side Buildkite conditional evaluator.

The current repo can parse and evaluate many documented examples:

- Comparators: `==`, `!=`, `=~`, `!~`.
- Logical operators: `||`, `&&`.
- Literals: integers, strings, booleans, and `null`.
- Parentheses, `!`, dotted object lookup, arrays, and `includes`.
- Regex literals, escaped `/`, `i` flags, RE2 compatibility mode, and bounded
  regexp2 matching.

Some of that current behavior is a useful foundation; some is divergent from
the server and should be removed from the Buildkite surface. The important gaps
are:

- The server grammar treats dotted names as flat identifiers. `build.env` is a
  function name and `build.branch` is an assigned variable name. The local
  object-lookup model is not enough for exact parity.
- The server parser supports ternary expressions and shell-style environment
  substitution forms such as `$branch`, `${branch:-fallback}`, and substring
  operations. Those are currently missing locally.
- The docs define many variables under `build`, `pipeline`, `organization`, and
  notification-only `step`, but this repo has no Buildkite-specific context
  builder or availability rules.
- The docs distinguish pipeline-level, step-level, build notification, and step
  notification conditionals. The current evaluator has no context kind, so it
  cannot reject or null-fill variables based on where the conditional runs.
- Missing documented nullable values need Buildkite-compatible behavior. Today a
  missing property is an error, while many documented variables should be `null`
  in specific contexts.
- The library has no stable root package API that consistently handles parser
  errors, evaluator errors, validation errors, and non-boolean final results.
- The local regex engine currently accepts some syntax the server rejects. Exact
  parity requires the Go library to reject server-rejected regex features even
  when regexp2 can evaluate them.
- The existing `@>` operator and any other non-server syntax should be removed
  from the Buildkite language surface instead of preserved as compatibility
  extensions.
- There is no local server-derived conformance corpus yet. The upstream
  `buildkite/buildkite` specs should be ported before inventing bespoke edge
  cases, otherwise each PR risks matching the docs examples but drifting from
  production behavior.

## Goals

- Match the server-side Buildkite conditional grammar, including syntax that is
  covered by upstream server specs but not emphasized in the public docs.
- Match variable availability, type checking, enum validation, nullable values,
  and evaluation behavior for every server context this library supports.
- Make `env()` and `build.env()` work with server-compatible validation and
  return semantics.
- Preserve fail-closed behavior: parse errors, unsupported syntax, unsupported
  flags, non-boolean final values, and context-ineligible variables must not
  silently evaluate to `true`.
- Add durable table-driven conformance tests that include every docs example
  plus upstream Buildkite server cases.
- Publish a small, idiomatic root Go package API for validation and evaluation.
- Keep implementation packages cohesive and testable, with clear
  single-responsibility boundaries and small interfaces only at consuming
  boundaries.
- Keep CI simple: `mise run check` should remain the default validation path.

## Non-Goals

- Do not model scheduler behavior, plugin execution, group-step expansion,
  dynamic pipeline uploads, branch filtering, or dependency behavior. The docs
  discuss those topics, but this repo should own expression parsing and
  evaluation, not pipeline scheduling.
- Do not implement Buildkite pipeline YAML parsing. The conditional parser
  should implement the server conditional grammar, including the shell-style
  substitution syntax that grammar accepts, but it should not become a YAML
  loader.
- Do not make live Buildkite API calls in default unit tests. Server oracle tests
  should be optional and clearly separated from deterministic local tests.
- Do not preserve divergent syntax for compatibility in the Buildkite evaluator.
  If a feature is not accepted by the server-side conditional language, remove
  it or keep it behind a clearly separate non-Buildkite internal test path until
  it can be deleted.

## Target Model

Make the module root, `github.com/buildkite/conditional`, the polished public
library surface. The root package should expose the Buildkite contract in terms
callers care about: validate this conditional, evaluate it in this Buildkite
context, and receive a boolean or a typed error.

Public shape:

```go
type ContextKind string

const (
	ContextPipelineWebhook     ContextKind = "pipeline_webhook"
	ContextStepIf              ContextKind = "step_if"
	ContextBuildNotification   ContextKind = "build_notification"
	ContextStepNotification    ContextKind = "step_notification"
)

type BuildkiteContext struct {
	Kind         ContextKind
	Build        Build
	Pipeline     Pipeline
	Organization Organization
	Step         *Step
	Env          map[string]string
}

func Validate(expression string, ctx BuildkiteContext) error
func Evaluate(expression string, ctx BuildkiteContext) (bool, error)
```

The API should do the server work callers should not have to remember:

- Parse the server grammar, including comments, ternaries, regex literals,
  flat dotted identifiers, and shell-style environment substitutions.
- Type-check the expression against the selected Buildkite context.
- Build the server-compatible assignment table for documented variables and
  functions.
- Evaluate to a final boolean and fail closed for every parser, validation,
  evaluation, regex, and non-boolean result error.

Implementation packages should have clear responsibilities:

- `lexer` tokenizes the server grammar.
- `parser` parses tokens into AST nodes and preserves source positions.
- `ast` models syntax, not evaluation policy.
- `evaluator` evaluates a type-checked AST against a server-style context.
- `object` or its replacement represents runtime values and type information.
- A Buildkite context package owns variable/function assignment construction,
  enum definitions, and context availability.

Codebase cleanup should push toward idiomatic Go package boundaries:

- Prefer concrete types in the public API. Introduce interfaces only where a
  consumer needs substitution, such as an optional server-oracle checker.
- Keep parser, evaluator, regex validation, context construction, and
  environment substitution as separate reasons to change.
- Move implementation-only packages under `internal/` as part of the breaking
  cleanup. The root package is the supported library API.
- Remove or isolate any generic-language behavior that conflicts with the
  server grammar.

## Server Sources To Match

Treat these upstream files as the parity contract when they differ from local
behavior:

- `buildkite/buildkite:app/models/conditional/grammar.kpeg`
  defines parser syntax. Important implications: dotted variable and function
  names are flat identifiers; `build.env("NAME")` is a function named
  `build.env`, not a method call through an object receiver; ternary `? :` and
  shell-style environment substitutions are server grammar.
- `buildkite/buildkite:app/models/conditional/regexp.rb`
  defines accepted regex flags and rejected regex features. The Go
  implementation may use regexp2, but it must reject syntax the server rejects.
- `buildkite/buildkite:app/models/conditional/type_check_visitor.rb`,
  `evaluation_visitor.rb`, `variable.rb`, `function.rb`, `enum.rb`, and
  `context.rb` define type checking, evaluation, functions, enums, nullable
  values, and assignment lookup.
- `buildkite/buildkite:app/models/build/condition.rb`
  defines the Buildkite assignment table, `env()` versus `build.env()`,
  Buildkite enum values, context construction, and nullable build data.
- The upstream specs under `spec/models/conditional`,
  `spec/models/build/condition_spec.rb`, notification specs, and
  `spec/validators/build_condition_validator_spec.rb` are the primary source of
  conformance cases to port.

## Current State

### Landed

- Modern Go structure is in place: `go.mod`, `cmd/conditional`, `mise.toml`, and
  `.buildkite/pipeline.yml`.
- `mise run check` runs `go vet`, `go build`, `go test`, and `staticcheck`.
- Core docs syntax is mostly implemented:
  - `==`, `!=`, `=~`, `!~`
  - `||`, `&&`
  - integers, strings, booleans, `null`
  - parentheses and `!`
  - `includes`
  - arrays
  - comments
  - regex literals with escaped `/`, `i` flags, regexp2, RE2 compatibility mode,
    and match timeout
- Parser rejects trailing tokens after a complete expression.
- Lexer handles unterminated regex literals without panicking.
- README documents the current regex interpolation boundary.
- Some landed behavior is now explicitly provisional because it diverges from
  the server grammar or server regex validator.

### Known Gaps

| Area | Current Behavior | Required Direction |
| --- | --- | --- |
| Dotted names | Local parsing/evaluation leans on nested object lookup. | Match the server grammar's flat dotted identifiers for variables and functions. |
| `build.env()` | Only top-level calls evaluate cleanly. `build.env("NAME")` currently fails during evaluation. | Treat `build.env` as a flat function identifier with server-compatible nullable return behavior. |
| Ternary syntax | Not implemented. | Implement server ternary `condition ? true_value : false_value` precedence and type checking. |
| Shell substitution | Not implemented. | Implement server grammar for `$name`, `${name}`, default/alternate/error forms, and substring forms. |
| Scope | Callers pass arbitrary `object.Struct`. | Add server-style Buildkite assignment tables with documented variables and context availability. |
| Nullable values | Missing nested properties error. | Documented nullable variables should be present as `null` for the right contexts. Truly unknown properties should still fail closed. |
| Context restrictions | No context kind. | Enforce pipeline, step, build-notification, and step-notification variable availability. |
| Final result | `Eval` returns any `object.Object`. Tests often ignore parser errors. | Public Buildkite evaluation should return `(bool, error)` and treat non-boolean results as errors. |
| Regex syntax | regexp2 accepts some features the server rejects. | Keep regexp2 only with a server-compatible validator for flags and unsupported constructs. |
| Divergent operators | `@>` exists locally but is not server syntax. | Remove it from the Buildkite language and tests. |
| Type mismatch semantics | Local behavior exists but is not server-proven. | Build a server-derived matrix for equality, regex matching, `includes`, `!`, missing values, and function argument errors. |
| Conformance | Unit tests cover selected docs examples. | Add idiomatic table-driven Go conformance tests that can be run locally and optionally compared with a server oracle. |

## Server Syntax And Context Surface To Cover

The public docs define these expression features, and they must all be covered:

- Comparators: `==`, `!=`, `=~`, `!~`.
- Logical operators: `||`, `&&`.
- Array operator: `includes`.
- Integers, strings, `true`, `false`, `null`.
- Parentheses and `!`.
- Regular expressions, including RHS-only regex matching and escaping `$` anchors
  in pipeline YAML to avoid interpolation.
- `//` comments.

The upstream server grammar and specs add syntax that is also in scope for exact
server parity:

- Ternary expressions: `condition ? true_value : false_value`.
- Dotted variable identifiers as flat assignment names, such as `build.branch`,
  `pipeline.slug`, and `organization.slug`.
- Dotted function identifiers as flat function names, such as `build.env`.
- Shell-style environment substitution in expressions and double-quoted strings:
  `$name`, `${name}`, `${name?}`, `${name:?}`, `${name-default}`,
  `${name:-default}`, `${name+alternate}`, `${name:+alternate}`, and substring
  forms such as `${name:1:2}`.
- Server string escape behavior for single-quoted strings, double-quoted
  strings, and substitution fallback strings.
- Server type checking for strings, numbers, booleans, arrays, nulls, regexes,
  typed variables, functions, and enums.

The docs and upstream `Build::Condition` context define this variable and
function surface:

- Common build variables: author, branch, commit, creator, id, message, number,
  pull request data, merge queue data, source, source event/action, state, tag.
  Author values are unverified webhook data; creator team values depend on
  Buildkite being able to identify a verified user.
- `build.env("NAME")`, including documented `BUILDKITE_*` variables and caller
  supplied custom variables.
- Pipeline variables: default branch, id, repository, slug, started failing, and
  started passing.
- Organization variables: id and slug.
- Step notification variables: id, key, label, type, state, and outcome.

The plan should not assume all variables are valid in all contexts. The docs
explicitly call out context-specific behavior, such as `build.state` for
notification-level conditionals and `step.*` for step notifications.

## Go Design Constraints

The final codebase should be a small Go library, not a transliteration of the
Ruby object model. Apply SOLID principles in idiomatic Go terms:

- Single responsibility: lexer, parser, regex validation, type checking,
  evaluation, environment substitution, and Buildkite context construction each
  get one reason to change.
- Open/closed: add new server variables, functions, and enum values through
  explicit assignment/type definitions and conformance tests, not by widening
  the evaluator to accept arbitrary unknown names.
- Liskov/interface substitution: avoid broad exported interfaces. Where
  substitution matters, such as an optional server oracle, define the smallest
  consumer-owned interface.
- Interface segregation: public callers should not need lexer/parser/evaluator
  internals to evaluate a conditional.
- Dependency inversion: the public evaluator depends on a Buildkite context
  contract; optional live server checks depend on an oracle interface, not on
  hard-coded network calls in unit tests.

Additional Go constraints:

- Exported identifiers in the root package need doc comments and stable error
  behavior.
- Error tests should use typed errors or error categories, plus source location
  where meaningful. Do not couple the Go suite to every word of a Ruby error
  unless exact message parity becomes a requirement.
- Test helper packages should stay small. Prefer local helper functions with
  `t.Helper()` over a custom testing framework.
- Do not introduce abstractions just to mirror Ruby classes. Add them only when
  they remove real duplication or protect the public API from implementation
  churn.

## Delivery Strategy

### Slice 1: Idiomatic Go Conformance Test Suite

Create the test shape before more feature work. Keep the test data in Go code so
cases are easy to read, refactor, and debug with normal `go test` output. Do not
add YAML files as test data.

Proposed files:

- Root package behavior tests:
  - `conditional_test.go`
  - `syntax_test.go`
  - `eval_test.go`
  - `context_test.go`
  - `regex_test.go`
- Package-local tests should remain where they are useful for narrow parser,
  lexer, or evaluator failures.
- A tiny test helper file is acceptable if it removes real duplication. Avoid a
  custom test framework.

Table shape:

```go
tests := []struct {
	name       string
	source     string
	expression string
	context    ContextKind
	build      Build
	env        map[string]string
	want       bool
	wantError  errorKind
}{
	{
		name:       "docs branch starts with features slash",
		source:     "docs/pipelines/configure/conditionals",
		expression: `build.branch =~ /^features\//`,
		context:    ContextStepIf,
		build:      Build{Branch: "features/foo"},
		want: true,
	},
	{
		name:       "missing tag is null",
		source:     "buildkite/buildkite spec/models/conditional/evaluator_spec.rb",
		expression: `build.tag == null`,
		context:    ContextStepIf,
		build:      Build{},
		want: true,
	},
}
```

Upstream sources to port:

- `buildkite/buildkite:spec/models/conditional/parser_spec.rb`
  for comments, operator precedence, negation, complex strings, regex literals,
  arrays, `includes`, parser error categories, and token-position-sensitive
  syntax cases.
- `buildkite/buildkite:spec/models/conditional/evaluator_spec.rb`
  for boolean/null/string comparisons, arrays, regex matching, function calls,
  ternary conditionals, environment-variable interpolation, and enum behavior.
- `buildkite/buildkite:spec/models/conditional/variable_spec.rb`
  for typed variables, nullable typed values, enum validation, and lazy values.
- `buildkite/buildkite:spec/models/build/condition_spec.rb`
  for `env()`, `build.env()`, `organization.*`, `pipeline.*`, `build.*`,
  pull request label behavior, webhook source event/action behavior, project env
  merging, validation rules, and the server context builder.
- `buildkite/buildkite:spec/validators/build_condition_validator_spec.rb`
  for blank conditionals, invalid expressions, and step-variable availability
  when the validator is configured for step context.
- `buildkite/buildkite:spec/models/build/notification_spec.rb` and
  `buildkite/buildkite:spec/models/step/notification_spec.rb`
  for fail-closed notification behavior and `step.*` availability.
- `buildkite/buildkite:spec/models/build/pipeline_config/build_notifications_spec.rb`
  and
  `buildkite/buildkite:spec/models/build/pipeline_config/step_notifications_spec.rb`
  should be mined once notification parsing is in scope.

Porting order:

1. Direct expression semantics: comments, precedence, negation, booleans,
   nulls, strings, numbers, arrays, `includes`, escaped regex delimiters, regex
   flags, ternaries, and short-circuiting.
2. Shell substitution semantics from the server evaluator: set, unset, empty,
   default, alternate, required, substring, nested fallback, and bad substring
   length cases.
3. Negative parser, type-checker, and evaluator cases represented as stable Go
   error categories: wrong operators, unknown variables, invalid function calls,
   invalid regex literals, final non-boolean values, invalid `env()` arguments,
   enum mismatches, and maximum nesting.
4. Buildkite context cases:
   `organization.*`, `pipeline.*`, `build.*`, `env()`, `build.env()`, webhook
   event/action fields, pull request label fields, project env merging, step
   variables, notification contexts, nullable documented values, and enum
   validation.
5. Divergence tests: local-only syntax such as `@>` and regex features rejected
   by the server should have explicit rejection tests.

Definition of done:

- Every docs example expression is represented in table-driven tests.
- Ported upstream examples carry a `source` string that points back to the
  originating `buildkite/buildkite` spec file.
- Server-supported upstream cases are either passing tests or recorded in this
  plan as the next implementation slice. Do not land permanently skipped parity
  tests.
- Tests can assert parse errors, evaluation errors, and boolean results.
- Test helpers assert errors by kind and location where meaningful, not by
  brittle full-message string comparisons unless message parity is explicitly in
  scope.
- Existing unit tests still run through `mise run check`.
- The helper makes parser errors visible; tests must not evaluate a nil or
  erroneous AST by accident.

### Slice 2: Public API And Package Boundary

Add the root `conditional` package API and make implementation package ownership
clear.

Definition of done:

- `Validate(expression, ctx)` and `Evaluate(expression, ctx)` exist in the root
  package.
- Exported context structs and enums have doc comments and stable field names.
- Parser, type-checker, evaluator, and regex errors are returned as typed Go
  errors or error categories.
- Non-boolean final objects are errors.
- Table-driven conformance tests use the root API for parity assertions.
- Implementation packages are moved under `internal/` so the root package is the
  only supported library API.

### Slice 3: Parser Grammar Parity

Make the parser match `app/models/conditional/grammar.kpeg`.

Definition of done:

- Dotted variable identifiers parse as flat assigned-variable names.
- Dotted function identifiers parse as flat function names.
- Ternary expressions parse with server precedence.
- Shell substitution syntax parses in operands, double-quoted strings, fallback
  strings, and substring arguments.
- String escape behavior matches the server grammar.
- Regex literal parsing matches server delimiters and the optional `i` flag.
- `@>` and any other non-server operators are removed from the Buildkite
  grammar.
- Parser no-panic tests cover unterminated strings, regexes, comments,
  substitutions, and deeply nested expressions.

### Slice 4: Type Checking And Evaluation Semantics

Port the server type-checker and evaluator behavior without copying the Ruby
object model.

Definition of done:

- Equality, inequality, regex matching, `includes`, logical operators, `!`, and
  ternary evaluation match upstream specs.
- Arrays, nulls, booleans, strings, numbers, regexes, typed variables, functions,
  and enums are type-checked before evaluation.
- Unknown variables/functions, wrong arity, unsupported operators, incompatible
  comparisons, and non-boolean logical operands fail closed.
- Short-circuiting prevents skipped branches from evaluating missing variables
  or failing functions, matching server behavior.
- Environment substitution evaluates set, unset, empty, required, default,
  alternate, and substring forms the same way as the server.

### Slice 5: Buildkite Context And `env()` Semantics

Implement `Build::Condition` context behavior in Go.

Definition of done:

- Context structs cover every server assignment in `Build::Condition`, including
  deprecated-but-server-supported values such as `build.fixed` if the server
  still exposes them.
- Nullable documented variables are materialized as `null` when valid but
  absent.
- Context-ineligible variables fail closed instead of being silently available.
- `env()` returns server-compatible strings and validation behavior.
- `build.env()` is a flat function identifier and returns server-compatible
  nullable values.
- `BUILDKITE_*` allowlist validation, typo suggestions, invalid names, and names
  starting with `$` match server error categories.
- Project env and build env merge semantics match `Build::PipelineEnvironment`.
- `step.*` only in step notification context.
- `build.pull_request.*` values on PR and non-PR builds.
- `build.merge_queue.*` values on merge queue and non-merge-queue builds.
- Verified and unverified actor cases for `build.author.*`,
  `build.creator.*`, `build.author.teams`, and `build.creator.teams`.
- Enumerated values for `build.source`, `build.state`, `step.type`,
  `step.state`, and `step.outcome`.
- `build.source_event`, `build.source_action`, and `build.pull_request.label`
  webhook behavior matches upstream tests.
- Verified-user-sensitive values and visible team lists are represented in the
  caller-provided context model without hard-coding server database behavior.

### Slice 6: Regex Exact Parity

Keep regexp2 as the matcher, but validate regex syntax against the server's
accepted feature set.

Required coverage:

- RHS-only regex matching.
- Escaped `/` delimiters.
- `i` flag.
- Unsupported flags.
- Server-rejected constructs from `Conditional::Regexp`, including lookbehind,
  negative lookbehind, atomic groups, possessive quantifiers, named captures, and
  conditionals.
- Server validation for shorthand character classes, POSIX classes, and any
  scanner cases where Ruby rejects syntax that regexp2 would accept.
- Literal dollar matches: `/\$/`, `/fee\$/`.
- Raw anchor matches: `/fee$/`, `/^v[0-9]+\.0$/`.

Definition of done:

- Test names distinguish regex anchors and literal-dollar matches from
  shell-substitution cases.
- Regex timeout behavior has a focused test that cannot make CI slow or flaky.
- The library rejects server-rejected regex features even if regexp2 can execute
  them.

### Slice 7: Divergence Removal And Codebase Cleanup

Remove syntax, tests, examples, and package shape that conflict with exact
server parity.

Definition of done:

- `@>` is removed from the Buildkite parser/evaluator/tests/docs unless upstream
  server evidence proves it is accepted.
- README examples are valid server conditionals. For example, remove or fix the
  current `meta-data("foo")` example because the docs say build meta-data is not
  available in conditional expressions.
- Nested object lookup is not part of the Buildkite evaluation surface unless it
  is only an internal representation of flat assignments.
- Public docs describe the root package API, context kinds, variable
  availability, supported syntax, and fail-closed behavior.
- Package boundaries are reviewed for cohesion, exported identifiers, comments,
  and unnecessary interfaces.

### Slice 8: Optional Server Oracle

Add a tool for checking table-driven conformance cases against Buildkite server
behavior.

Possible shape:

```sh
mise run conformance:check
```

The oracle should be optional because it may need Buildkite credentials, a test
pipeline, or private/internal access. Default CI should run committed Go tables
only.

Definition of done:

- The tool can evaluate the Go conformance cases against the server or a
  server-backed API and report mismatches.
- If recording observed server behavior is useful later, add generated output as
  a deliberate follow-up rather than as the default test data format.
- If a live oracle is not available, the plan records exactly which semantics
  remain inferred from docs rather than server-proven.

## Verification

Run on every implementation slice:

```sh
mise run check
git diff --check
```

Add these targeted checks as the plan lands:

- Root package table-driven tests for every supported server behavior.
- Package-local unit tests for lexer/parser/type-checker/evaluator behavior
  where failures are easier to diagnose below the public API.
- Ported conformance tests for docs examples and upstream server-derived edge
  cases.
- Fuzz tests or bounded randomized tests for lexer/parser no-panic behavior,
  especially regex literals, strings, comments, shell substitutions, ternaries,
  and nested expressions.
- Regression tests for short-circuiting so missing values in skipped branches do
  not fail evaluation.
- Regression tests for nullable documented variables versus unknown properties.
- Regex timeout test for a pathological regexp2 pattern, written with a short
  timeout and generous assertion so it is deterministic.
- Rejection tests for divergent local syntax and server-rejected regex features.
- `go test -run` examples in PR descriptions for narrow iteration, with
  `mise run check` as the final validation.
- Buildkite pipeline validation with `bk pipeline validate --file
  .buildkite/pipeline.yml` when CI config changes.

## Resolved Decisions

- The repo uses `master` as the default branch.
- `mise run check` is the canonical local validation command.
- Exact server-side Buildkite syntax and semantics are the target. Public docs
  are necessary coverage, but upstream server grammar/spec behavior is also in
  scope.
- Divergent local syntax can be removed. `@>` should not remain in the
  Buildkite language unless upstream server evidence proves it is accepted.
- Dotted names are flat server identifiers, not object/method syntax in the
  Buildkite grammar.
- Ternary expressions, shell-style environment substitutions, typed variables,
  enums, and dotted function names are required parity work because upstream
  server specs cover them.
- `regexp2` remains the regex engine, but the library must reject regex features
  the server rejects. The linear-time guarantee from Go's regexp engine is not a
  requirement, but regex matching must stay bounded by timeout.
- Keep regex literal semantics in the parser. Raw `$` is a regex anchor and
  `\$` is a literal dollar; shell substitution handling belongs to the server
  grammar, not heuristic regex rewriting.
- Port upstream Buildkite specs into plain table-driven Go tests before
  inventing bespoke parity cases. Do not use YAML test data for the conformance
  corpus.
- The root package is the supported Go library API. Existing subpackages can
  remain during transition, but they are not the final parity contract.
- Breaking API and package cleanup is allowed to reach the polished library
  shape, including moving implementation packages under `internal/`.
- Root-package `Validate(expression, ctx)` and `Evaluate(expression, ctx)` are
  the public API names.
- Error parity means exact accept/reject behavior, stable Go error categories,
  and source location where meaningful. Byte-for-byte Ruby error text is not a
  requirement for the first parity release.
- The library should derive pure conditional values, such as `source_event` from
  supplied environment data, but callers must provide database-backed facts such
  as visible teams and preferred emails.
- Implementation packages should move under `internal/` once the root API is in
  place.

## Deferred Work

- Optional live server oracle. Ported upstream specs should carry the first
  several implementation slices.
- Byte-for-byte error text if the first release can provide stable typed errors,
  source locations, and exact accept/reject behavior.

## Key Learnings From Pressure-Testing

- Exact parity cannot be proven from the public docs examples alone. The
  upstream grammar and specs define real server syntax, including ternary
  expressions and shell-style environment substitutions, that must be included.
- Dotted identifiers are a larger architectural issue than `build.env()` alone.
  The server uses flat assignment/function names; a nested object lookup model
  will keep producing edge-case drift.
- Regex `$` handling should not be solved with parser heuristics. Raw `$` and
  `\$` have normal regex meaning inside regex literals; shell substitution is a
  separate part of the server grammar.
- The current generic scope API makes simple tests easy but hides Buildkite's
  nullable, typed, enum, and context-specific variable rules. A server-style
  Buildkite context builder is the smallest useful abstraction for parity.
- Table-driven conformance tests should land before more behavior changes. They
  give every follow-up PR a durable place to encode docs examples, server
  observations, and regressions.
- The upstream `buildkite/buildkite` specs provide the best available parity
  corpus. Porting those examples into Go tables should be the default source of
  new tests, with invented cases reserved for gaps the upstream specs and docs
  do not cover.
- SOLID in this repo should mean cohesive Go package responsibilities and a
  small public root API. It should not mean adding broad interfaces or a Ruby-like
  class structure.
