---
status: proposed
last_reviewed: 2026-06-06
spec_refs:
  - https://buildkite.com/docs/pipelines/configure/conditionals
---

# Buildkite Conditionals Parity

## Summary

This plan brings `buildkite/conditional` to exact parity with the server-side
conditional language documented in Buildkite's conditionals guide. The current
repository is now a modern Go module with `mise` tasks, CI wiring, core boolean
syntax, `null`, `includes`, short-circuiting logical operators, and regexp2-based
regular expressions. The remaining work is less about adding isolated operators
and more about making the evaluator behave like Buildkite in the contexts where
Buildkite evaluates conditionals.

The target outcome is a small, well-tested library that can answer the same
question as Buildkite: given a conditional expression, a Buildkite evaluation
context, and the source string form that Buildkite would evaluate, does the
expression evaluate to `true`, `false`, or a fail-closed error?

The hard part is parity evidence. The public docs list syntax, variables, and
examples, but some semantics are only observable from the server evaluator:
missing values, context-specific variables, type mismatches, function receiver
behavior, and the boundary between YAML/environment interpolation and regex
parsing. The upstream `buildkite/buildkite` repo already has RSpec coverage for
much of this behavior. This plan therefore treats ported table-driven Go
conformance tests and an optional server oracle as first-class deliverables, not
test polish after implementation.

## Problem

The library currently evaluates a generic expression language with a generic
`object.Struct` scope. That is useful, but it is not yet the same thing as the
server-side Buildkite conditional evaluator described by the docs.

The current repo can parse and evaluate many documented examples:

- Comparators: `==`, `!=`, `=~`, `!~`.
- Logical operators: `||`, `&&`.
- Literals: integers, strings, booleans, and `null`.
- Parentheses, `!`, dotted object lookup, arrays, and `includes`.
- Regex literals, escaped `/`, `i` flags, RE2 compatibility mode, and bounded
  regexp2 matching.

The gaps are now mostly semantic:

- The docs expose `build.env("NAME")`, but the evaluator only supports top-level
  function calls. `build.env("NAME")` parses as a dotted call and currently fails
  during evaluation.
- The docs define many variables under `build`, `pipeline`, `organization`, and
  notification-only `step`, but this repo has no Buildkite-specific context
  builder or availability rules.
- The docs distinguish pipeline-level, step-level, build notification, and step
  notification conditionals. The current evaluator has no context kind, so it
  cannot reject or null-fill variables based on where the conditional runs.
- Missing documented nullable values need Buildkite-compatible behavior. Today a
  missing property is an error, while many documented variables should be `null`
  in specific contexts.
- The library has no stable public "parse and evaluate to bool" API that
  consistently handles parser errors, evaluator errors, and non-boolean results.
- The documentation around regex `$` escaping is tied to Buildkite interpolation.
  The evaluator now preserves regex semantics, where raw `$` is an anchor and
  `\$` is a literal dollar. Exact server parity still needs tests that prove the
  string seen by the server evaluator after interpolation.
- There is no local server-derived conformance corpus yet. The upstream
  `buildkite/buildkite` specs should be ported before inventing bespoke edge
  cases, otherwise each PR risks matching the docs examples but drifting from
  production behavior.

## Goals

- Match the documented conditional syntax from the Buildkite docs.
- Match documented variable availability and values for every conditional
  context the docs describe.
- Make `build.env()` work with the documented environment variable allowlist and
  custom environment variables when the caller provides them.
- Preserve fail-closed behavior: parse errors, unsupported syntax, unsupported
  flags, non-boolean final values, and context-ineligible variables must not
  silently evaluate to `true`.
- Add durable table-driven conformance tests that include every docs example
  plus upstream Buildkite server cases that can be ported cleanly.
- Keep the library useful as a generic evaluator while adding Buildkite-specific
  helpers behind an explicit package or API boundary.
- Keep CI simple: `mise run check` should remain the default validation path.

## Non-Goals

- Do not model scheduler behavior, plugin execution, group-step expansion,
  dynamic pipeline uploads, branch filtering, or dependency behavior. The docs
  discuss those topics, but this repo should own expression parsing and
  evaluation, not pipeline scheduling.
- Do not implement Buildkite YAML parsing as the default expression evaluator.
  If interpolation support is needed, expose it as an explicit input mode or
  helper so raw expression evaluation keeps normal regex semantics.
- Do not make live Buildkite API calls in default unit tests. Server oracle tests
  should be optional and clearly separated from deterministic local tests.
- Do not remove the legacy `@>` operator immediately. It is not in the current
  docs, but existing users may rely on it. Treat it as a documented compatibility
  extension and keep its tests.

## Target Model

Add a Buildkite-specific evaluation layer over the existing lexer, parser,
evaluator, and object packages.

The core expression packages should stay mostly generic:

- `lexer` tokenizes expression strings.
- `parser` parses tokens into AST nodes.
- `evaluator` evaluates an AST with a `Scope`.
- `object` represents runtime values.

Add a package or top-level API that makes Buildkite context explicit:

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

func EvaluateBuildkite(expression string, ctx BuildkiteContext) (bool, error)
```

The API should do three things that callers should not have to remember:

- Build the documented variable scope for the selected context.
- Wire documented functions, especially `build.env()`.
- Enforce that the final result is a boolean.

Keep raw evaluator access for existing generic callers. The Buildkite-specific
API is the parity surface.

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

### Known Gaps

| Area | Current Behavior | Required Direction |
| --- | --- | --- |
| `build.env()` | Only top-level calls evaluate cleanly. Dotted calls fail because `.` expects an identifier on the right. | Support Buildkite method-style calls for documented receivers, starting with `build.env("NAME")`. |
| Scope | Callers pass arbitrary `object.Struct`. | Add Buildkite context builders with documented variables and context availability. |
| Nullable values | Missing nested properties error. | Documented nullable variables should be present as `null` for the right contexts. Truly unknown properties should still fail closed. |
| Context restrictions | No context kind. | Enforce pipeline, step, build-notification, and step-notification variable availability. |
| Final result | `Eval` returns any `object.Object`. Tests often ignore parser errors. | Public Buildkite evaluation should return `(bool, error)` and treat non-boolean results as errors. |
| Regex `$` interpolation | Raw evaluator preserves regex semantics. | Add table-driven tests for raw evaluator input and, if needed, a separate interpolation-aware entry point for YAML/upload strings. |
| Type mismatch semantics | Local behavior exists but is not server-proven. | Build a server-derived matrix for equality, regex matching, `includes`, `!`, missing values, and function argument errors. |
| Conformance | Unit tests cover selected docs examples. | Add table-driven conformance tests that can be run locally and optionally compared with a server oracle. |

## Buildkite Docs Surface To Cover

The current docs define these expression features:

- Comparators: `==`, `!=`, `=~`, `!~`.
- Logical operators: `||`, `&&`.
- Array operator: `includes`.
- Integers, strings, `true`, `false`, `null`.
- Parentheses and `!`.
- Regular expressions, including RHS-only regex matching and escaping `$` anchors
  in pipeline YAML to avoid interpolation.
- `//` comments.

The docs also define context and variable surface:

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

## Delivery Strategy

### Slice 1: Table-Driven Conformance Tests

Create table-driven Go conformance tests before more feature work. Keep the
test data in code so it is easy to read, refactor, and debug with normal Go test
output. Do not add YAML files as test data.

Proposed files:

- `conformance/conformance_test.go`
- `conformance/testcase.go` if shared helpers make the table clearer

Table shape:

```go
tests := []struct {
	name       string
	source     string
	expression string
	context    ContextKind
	scope      object.Struct
	want       bool
	wantError  bool
}{
	{
		name:       "docs branch starts with features slash",
		source:     "docs/pipelines/configure/conditionals",
		expression: `build.branch =~ /^features\//`,
		context:    ContextStepIf,
		scope: object.Struct{
			"build": object.Struct{
				"branch": &object.String{Value: "features/foo"},
			},
		},
		want: true,
	},
	{
		name:       "missing tag is null",
		source:     "buildkite/buildkite spec/models/conditional/evaluator_spec.rb",
		expression: `build.tag == null`,
		context:    ContextStepIf,
		scope: object.Struct{
			"build": object.Struct{
				"tag": &object.Null{},
			},
		},
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

1. Direct expression semantics that are already supported or immediately
   planned: comments, precedence, negation, booleans, nulls, strings, arrays,
   `includes`, escaped regex delimiters, regex flags, and short-circuiting.
2. Negative parser and evaluator cases that can be represented as stable error
   categories: wrong operators, unknown variables, invalid function calls,
   invalid regex literals, final non-boolean values, and invalid `env()`
   arguments.
3. Buildkite context cases once the context builder exists:
   `organization.*`, `pipeline.*`, `build.*`, `build.env()`, webhook event and
   action fields, pull request label fields, project env merging, step
   variables, notification contexts, and nullable documented values.
4. Upstream-only or not-yet-local syntax should be classified before it is
   implemented: ternary `? :`, shell-style environment interpolation such as
   `$branch` and `${branch:-fallback}`, typed variables/enums, dotted function
   names, token-position assertions, maximum nesting validation, and exact
   server regex restrictions.

Definition of done:

- Every docs example expression is represented in table-driven tests.
- Ported upstream examples carry a `source` string that points back to the
  originating `buildkite/buildkite` spec file.
- Unsupported upstream cases are recorded as planned parity work rather than
  landed as skipped or permanently failing tests.
- Tests can assert parse errors, evaluation errors, and boolean results.
- Existing unit tests still run through `mise run check`.
- The helper makes parser errors visible; tests must not evaluate a nil or
  erroneous AST by accident.

### Slice 2: Public Buildkite Evaluation API

Add a small public API that owns parse errors, evaluation errors, and result type
checking.

Definition of done:

- `EvaluateBuildkite(expression, ctx)` returns `(bool, error)`.
- Parser errors are joined into a useful error.
- Evaluator errors are returned as errors.
- Non-boolean final objects are errors.
- Existing generic package APIs remain available.
- Table-driven conformance tests use this API for Buildkite parity tests.

### Slice 3: Buildkite Context Builder

Implement documented variable mapping for `build`, `pipeline`, `organization`,
and `step`.

Definition of done:

- Context structs cover every variable listed in the docs.
- Nullable documented variables are materialized as `object.Null` when absent in
  a context where they are valid.
- Context-ineligible variables fail closed instead of being silently available.
- Array variables such as `build.creator.teams`, `build.author.teams`, and
  `build.pull_request.labels` become `object.Array`.
- Verified-user-sensitive values are represented explicitly, including cases
  where the server cannot attach teams to the actor.
- The conformance tables cover every documented variable at least once.

### Slice 4: `build.env()` Semantics

Support the documented `build.env("NAME")` function.

Implementation options:

- Extend the AST/evaluator to support calls through a dotted receiver, such as
  `build.env("BUILDKITE_TAG")`.
- Or lower `build.env("NAME")` during evaluation into a scoped function lookup
  while keeping the parser AST unchanged.

Recommended direction: support method-style calls explicitly in the evaluator.
That matches the docs and avoids encoding a dotted function name into the lexer.

Definition of done:

- `build.env("BUILDKITE_TAG")` returns a string when present and `null` when
  absent.
- Custom variables are available when provided by the caller.
- Non-string arguments, wrong arity, and unknown function receivers fail closed.
- Docs examples using both `build.tag` and `build.env("BUILDKITE_TAG")` pass.

### Slice 5: Context Availability And Type Semantics

Lock down behavior that is easy to get subtly wrong.

Test matrix:

- `build.state` in build notification context versus step context.
- `pipeline.started_failing` and `pipeline.started_passing` in build notification
  context.
- `step.*` only in step notification context.
- `build.pull_request.*` values on PR and non-PR builds.
- `build.merge_queue.*` values on merge queue and non-merge-queue builds.
- Verified and unverified actor cases for `build.author.*`,
  `build.creator.*`, `build.author.teams`, and `build.creator.teams`.
- Enumerated values for `build.source`, `build.state`, `step.type`,
  `step.state`, and `step.outcome`.
- Equality across same and different types.
- `includes` for arrays of strings, integers, booleans, nulls, and mixed-type
  arrays if the server permits them.
- `!` on booleans, `null`, strings, integers, arrays, and missing values.
- Missing unknown properties versus documented nullable properties.

Definition of done:

- Local behavior is either server-proven or explicitly recorded as a chosen
  compatibility behavior.
- Error messages do not need byte-for-byte parity, but error categories should be
  stable enough for callers and tests.

### Slice 6: Regex And Interpolation Boundary

Keep regexp2 for syntax parity, with the accepted backtracking tradeoff bounded
by `MatchTimeout`.

Required coverage:

- RHS-only regex matching.
- Escaped `/` delimiters.
- `i` flag.
- Unsupported flags.
- POSIX classes in RE2 compatibility mode.
- Literal dollar matches: `/\$/`, `/fee\$/`.
- Raw anchor matches: `/fee$/`, `/^v[0-9]+\.0$/`.
- Docs/YAML examples that escape anchors before interpolation.

Recommended direction:

- Keep parser-level regex semantics exact: raw `$` is an anchor, `\$` is a
  literal dollar.
- Add an explicit interpolation-aware input mode only if the server oracle
  proves callers need to pass pre-interpolation condition strings into this
  library.
- Do not reintroduce heuristic parser rewriting of `\$`; it cannot distinguish
  anchored docs examples from legitimate literal-dollar regexes.

Definition of done:

- Test names distinguish raw evaluator expressions from Buildkite YAML/upload
  expressions.
- The README documents whichever input modes are supported.
- Regex timeout behavior has a focused test that cannot make CI slow or flaky.

### Slice 7: Optional Server Oracle

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

### Slice 8: Documentation And Compatibility Cleanup

Update public docs once the implementation and conformance tables settle.

Definition of done:

- README support list matches implemented parity.
- README examples are valid expressions. For example, remove or fix the current
  `meta-data("foo")` example because the docs say build meta-data is not
  available in conditional expressions.
- Document `@>` as a compatibility extension if it remains.
- Document context kinds and which variables are available in each.
- Document the regex input boundary clearly.

## Verification

Run on every implementation slice:

```sh
mise run check
git diff --check
```

Add these targeted checks as the plan lands:

- Unit tests for lexer/parser/evaluator behavior around each syntax feature.
- Table-driven conformance tests for docs examples and server-derived edge
  cases.
- Fuzz tests or bounded randomized tests for lexer/parser no-panic behavior,
  especially regex literals, strings, comments, and nested expressions.
- Regression tests for short-circuiting so missing values in skipped branches do
  not fail evaluation.
- Regression tests for nullable documented variables versus unknown properties.
- Regex timeout test for a pathological regexp2 pattern, written with a short
  timeout and generous assertion so it is deterministic.
- Buildkite pipeline validation with `bk pipeline validate --file
  .buildkite/pipeline.yml` when CI config changes.

## Resolved Decisions

- The repo uses `master` as the default branch.
- `mise run check` is the canonical local validation command.
- `regexp2` remains the regex engine for server-side syntax parity. The linear
  time guarantee from Go's regexp engine is not a requirement for this project,
  but regex matching must stay bounded by timeout.
- Keep raw expression regex semantics in the parser. Do not use heuristic
  rewriting of `\$` into `$`; handle interpolation as a separate explicit input
  concern if required.
- Port upstream Buildkite specs into plain table-driven Go tests before
  inventing bespoke parity cases. Do not use YAML test data for the conformance
  corpus.
- Keep `@>` for now as a compatibility extension, even though the docs now list
  `includes`.

## Open Questions

### Blocking First Implementation Slice

None. The first slice can land table-driven Go conformance tests for docs
examples and directly portable upstream cases without settling every semantic
edge.

### Needed Before Server-Parity Claim

- Which server surface can act as the oracle for expression evaluation? The
  default should be committed Go tests plus an optional check tool; a live server
  dependency should not be required in normal CI.
- Does the library need to accept pre-interpolation YAML/upload conditional
  strings, or only the expression string after Buildkite interpolation? The
  recommended default is two explicit modes if both are needed.
- Should context-ineligible documented variables return `null` or error? The
  recommended default is: valid-but-absent values return `null`; invalid for the
  context errors. Verify with the server oracle before declaring parity.
- Which upstream-only syntax is required for the public parity claim? The
  upstream server specs cover ternary `? :`, shell-style environment
  interpolation, typed variables/enums, dotted function names, and
  position-aware errors, but not all of that appears in the current public docs.
- Should regexp2 be restricted to the exact server-accepted regex feature set?
  The current Go implementation accepts a broader regex syntax than some
  upstream server tests expect, so these cases need either implementation
  restrictions or an explicit compatibility note before claiming exact parity.

### Safe To Defer

- Byte-for-byte error message parity. Error categories matter more than matching
  server text exactly.
- Removing or deprecating `@>`. Keep it until compatibility impact is known.
- Public API naming. The first implementation can use an internal harness and
  settle package names before exposing a stable top-level API.

## Key Learnings From Pressure-Testing

- Exact parity cannot be proven from the docs examples alone. The docs define
  syntax and public variables, but several important semantics need either a
  server oracle or explicitly recorded decisions.
- Regex `$` handling is the highest-risk ambiguity because docs examples are
  affected by interpolation while the parser also needs normal regex literal
  semantics. The plan avoids another heuristic parser rewrite and instead
  separates raw expression evaluation from interpolation-aware input handling.
- The current generic scope API makes simple tests easy but hides Buildkite's
  nullable and context-specific variable rules. A Buildkite context builder is
  the smallest useful abstraction for parity.
- Table-driven conformance tests should land before more behavior changes. They
  give every follow-up PR a durable place to encode docs examples, server
  observations, and regressions.
- The upstream `buildkite/buildkite` specs provide the best available parity
  corpus. Porting those examples into Go tables should be the default source of
  new tests, with invented cases reserved for gaps the upstream specs and docs
  do not cover.
